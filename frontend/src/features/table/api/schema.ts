import { useCallback, useMemo } from "react";
import { useQueries, type UseQueryResult } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { keys } from "@/shared/lib/query-keys";
import { errorMessage } from "@/shared/lib/toast";
import { EMPTY_SCHEMA, type Field, type Relation, type TableSchema } from "../model/types";
import type { FieldsResponseDto, RelationDto, RelationsResponseDto } from "./dto";
import { toField, toRelation } from "./normalize";

/**
 * Схема таблицы: поля и связи.
 *
 * Три запроса и один хук: поля, список связей и — по одному на связь —
 * её настройки. Половина схемы вызывающему коду бесполезна: без связей
 * нельзя нарисовать колонку-ссылку, а без настроек связи в ней нечего
 * показать, кроме uuid.
 *
 * Почему запрос на каждую связь, а не один общий: список связей
 * (GET /v2/relations/{slug}) отдаёт в `view_fields` НЕ настроенные поля
 * показа, а само поле-связь — там `jsonb_agg(field.*)` по джойну
 * `field.relation_id = relation.id`. Настроенный список полей лежит
 * в колонке `relation.view_fields` и возвращается только поштучно,
 * из GET /v2/relations/{slug}/{id}. Связей у таблицы единицы, запросы
 * идут параллельно и живут в кэше пять минут — это дешевле, чем
 * показывать пользователю пустые колонки.
 *
 * staleTime большой: схему меняет админ в конструкторе, а не
 * пользователь при работе со строками.
 */
const SCHEMA_STALE = 5 * 60_000;

type FieldsQueries = [
  UseQueryResult<FieldsResponseDto, Error>,
  UseQueryResult<RelationsResponseDto, Error>,
];

type RelationQueries = UseQueryResult<RelationDto, Error>[];

export function useTableSchema(tableSlug: string | undefined, columnIds?: string[]) {
  const slug = tableSlug ?? "";

  const base = useQueries({
    queries: [
      {
        queryKey: keys.tables.fields(slug),
        queryFn: () => api.get<FieldsResponseDto>(`/v2/fields/${slug}`),
        enabled: Boolean(slug),
        staleTime: SCHEMA_STALE,
      },
      {
        queryKey: keys.tables.relations(slug),
        queryFn: () => api.get<RelationsResponseDto>(`/v2/relations/${slug}`),
        enabled: Boolean(slug),
        staleTime: SCHEMA_STALE,
      },
    ],
    combine: useCallback(
      ([fields, relations]: FieldsQueries) => ({
        schema: toSchema(fields.data, relations.data, slug),
        isLoading: fields.isLoading || relations.isLoading,
        /** Причина отказа словами. null — всё в порядке. */
        error: errorMessage(fields.error ?? relations.error, "table.loadFailed"),
        refetch: () => {
          void fields.refetch();
          void relations.refetch();
        },
      }),
      [slug],
    ),
  });

  /*
   * Настройки нужны не всем связям, а только тем, чьи колонки показаны:
   * у таблицы их бывает полтора десятка, а во view выведены три, и
   * остальные тринадцать запросов уходят в никуда на каждую загрузку
   * страницы. View перечисляет колонки-связи id СВЯЗИ — по нему и
   * отбираем.
   */
  const shown = columnIds ? new Set(columnIds) : undefined;
  const needed = shown
    ? base.schema.relations.filter((relation) => shown.has(relation.id))
    : base.schema.relations;

  const details = useQueries({
    queries: needed.map((relation) => ({
      queryKey: keys.tables.relation(slug, relation.id),
      queryFn: () => api.get<RelationDto>(`/v2/relations/${slug}/${relation.id}`),
      staleTime: SCHEMA_STALE,
    })),
    /*
     * Список из ответов → словарь настроек. Связь без ответа (ещё едет
     * или отказ) остаётся с тем, что дал общий список.
     *
     * Ссылка постоянная: react-query пересобирает результат `combine`
     * при смене его identity, то есть со стрелкой на месте — каждый
     * рендер, и схема таблицы каждый раз оказывалась новым объектом.
     */
    combine: useCallback(
      (results: RelationQueries) =>
        new Map(
          results
            .map((result) => result.data)
            .filter((dto): dto is RelationDto => Boolean(dto?.id))
            .map((dto) => [dto.id!, toRelation(dto, slug)] as const),
        ),
      [slug],
    ),
  });

  /*
   * Ссылка на схему держится, пока не приехали новые данные. Без этого
   * `relations` пересобирались на каждый рендер, а на них завязаны
   * вкладки карточки и колонки-ссылки: useMemo у вызывающих не спасал —
   * зависимость менялась вместе с рендером.
   */
  const schema = useMemo(() => withDetails(base.schema, details), [base.schema, details]);

  return { schema, isLoading: base.isLoading, error: base.error, refetch: base.refetch };
}

/** Настройки связи поверх её короткой формы из списка. */
function withDetails(schema: TableSchema, details: Map<string, Relation>): TableSchema {
  if (!details.size) return schema;

  return {
    ...schema,
    relations: schema.relations.map((relation) => details.get(relation.id) ?? relation),
  };
}

export function toSchema(
  fields: FieldsResponseDto | undefined,
  relations: RelationsResponseDto | undefined,
  tableSlug: string,
): TableSchema {
  if (!fields && !relations) return EMPTY_SCHEMA;

  const list = (relations?.relations ?? [])
    .filter((dto) => dto.id)
    .map((dto) => toRelation(dto, tableSlug));

  return {
    fields: nameRelations(
      (fields?.fields ?? []).filter((dto) => dto.id).map(toField),
      list,
    ),
    relations: list,
  };
}

/**
 * Безымянной колонке-связи имя даёт ЧУЖАЯ таблица.
 *
 * У связи подпись лежит в `attributes.label`, и когда её не задали,
 * колонка называется своим слагом — `orders_id`. Человеку он говорит
 * про устройство базы, а не про то, что в колонке; «Заказы» говорит.
 * Случай не редкий: обратную сторону связи бэкенд заводит сам и
 * без имени вовсе.
 *
 * Подставляется только когда своего имени нет: заданное админом
 * перебивать нечем.
 */
function nameRelations(fields: Field[], relations: Relation[]): Field[] {
  const byId = new Map(relations.map((relation) => [relation.id, relation]));

  return fields.map((field) => {
    if (!field.relationId || field.label !== field.slug) return field;

    const relation = byId.get(field.relationId);
    if (!relation?.toLabel) return field;

    return { ...field, label: relation.toLabel, labels: relation.toLabels };
  });
}
