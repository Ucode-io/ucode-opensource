import { useInfiniteQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { reportError, toast } from "@/shared/lib/toast";
import {
  RELATION_DIRECTION,
  toAutoFiltersBody,
  toSelfDefaultBody,
  type RelationDraft,
} from "../model/relation-draft";
import type { Labels, Relation } from "../model/types";
import { invalidateSchema } from "./fields";
import { pickLabels } from "./normalize";

/** Одна таблица в списке. Кроме слага, подписи и значка отсюда ничего не нужно. */
type TableDto = {
  id?: string;
  slug?: string;
  label?: string;
  /** Значок таблицы в том же формате, что у пунктов меню: см. DynamicIcon. */
  icon?: string;
  attributes?: Record<string, unknown>;
};

type TablesResponseDto = { tables?: TableDto[]; count?: number };

export type TableOption = {
  /** Идентификатор таблицы: связям хватает слага, а шаблону нужен id. */
  id: string;
  slug: string;
  /** Базовая подпись — колонка `label`. Показывать её напрямую нельзя. */
  label: string;
  /** Подписи по языкам ДАННЫХ: attributes.label_<код>. */
  labels: Labels;
  icon: string;
};

/** Строк на страницу в списках выбора. Одна прокрутка — одна страница. */
const PAGE = 30;

/**
 * Список таблиц проекта — чтобы выбрать, куда ведёт связь.
 *
 * Постранично: в проекте бывают сотни таблиц, и тянуть их все ради
 * выпадающего списка незачем. Поиск отдан серверу — он ищет по всем,
 * а не по загруженной странице.
 *
 * Следующая страница просится по смещению: ручка отдаёт `count`, но
 * не отдаёт курсора, и «дальше» здесь — это offset прочитанного.
 */
export function useTables(search: string) {
  const projectId = useSession().getProjectId() ?? "";
  const query = search.trim();

  const result = useInfiniteQuery({
    queryKey: [...keys.tables.list(projectId), query],
    initialPageParam: 0,
    queryFn: ({ pageParam }) =>
      api.get<TablesResponseDto>("/v2/collections", {
        params: { limit: PAGE, offset: pageParam, ...(query ? { search: query } : {}) },
      }),
    // Страница короче запрошенной — она последняя, дальше просить нечего.
    getNextPageParam: (last, pages) =>
      (last.tables ?? []).length < PAGE ? undefined : pages.length * PAGE,
    enabled: Boolean(projectId),
    // Таблицы заводит админ в конструкторе, а не пользователь по ходу работы.
    staleTime: 5 * 60_000,
  });

  const items: TableOption[] = (result.data?.pages ?? [])
    .flatMap((page) => page.tables ?? [])
    .filter((dto) => dto.slug)
    .map((dto) => ({
      id: dto.id ?? "",
      slug: dto.slug!,
      label: dto.label?.trim() || dto.slug!,
      /*
       * Язык не входит в ключ кэша: имена лежат в ответе все сразу,
       * и переключение языка данных не должно перезапрашивать список.
       * Выбирает нужное вызывающий — через localized().
       */
      labels: pickLabels(dto.attributes),
      icon: dto.icon ?? "",
    }));

  return {
    items,
    isLoading: result.isFetching,
    hasMore: result.hasNextPage,
    loadMore: () => void result.fetchNextPage(),
  };
}

/**
 * Поля ЧУЖОЙ таблицы постранично — из них выбирают, что показывать
 * вместо идентификатора связанной записи.
 *
 * Отдельно от useTableSchema: та тянет схему целиком вместе со связями
 * и их настройками, а здесь нужен плоский список с поиском. Размер
 * страницы задаёт сервер: `limit` в этой ручке он игнорирует и всегда
 * берёт сотню (field.go, GetAllFields), поэтому шаг смещения — сотня,
 * а не наш PAGE.
 */
const FIELDS_PAGE = 100;

export function useTableFields(tableSlug: string | undefined, search: string) {
  const slug = tableSlug ?? "";
  const query = search.trim();

  const result = useInfiniteQuery({
    queryKey: [...keys.tables.fields(slug), "page", query],
    initialPageParam: 0,
    queryFn: ({ pageParam }) =>
      api.get<FieldsPageDto>(`/v2/fields/${slug}`, {
        params: { offset: pageParam, ...(query ? { search: query } : {}) },
      }),
    getNextPageParam: (last, pages) =>
      (last.fields ?? []).length < FIELDS_PAGE ? undefined : pages.length * FIELDS_PAGE,
    enabled: Boolean(slug),
    staleTime: 5 * 60_000,
  });

  const items = (result.data?.pages ?? [])
    .flatMap((page) => page.fields ?? [])
    .filter((dto) => dto.id)
    .map((dto) => ({ id: dto.id!, label: pickFieldLabel(dto), slug: dto.slug ?? "" }));

  return {
    items,
    isLoading: result.isFetching,
    hasMore: result.hasNextPage,
    loadMore: () => void result.fetchNextPage(),
  };
}

type FieldPageDto = {
  id?: string;
  slug?: string;
  label?: string;
  type?: string;
  attributes?: Record<string, unknown>;
};

type FieldsPageDto = { fields?: FieldPageDto[] };

/**
 * Подпись поля: та же логика, что и в normalize.pickLabel, но по сырому
 * ответу — гонять страницу через полную нормализацию ради одной строки
 * незачем.
 *
 * У полей-связей колонка `label` содержит не подпись, а служебное имя,
 * которое бэкенд собирает сам: «FROM example_hey TO listings». Показывать
 * его человеку нельзя, и настоящее имя лежит в `attributes.label`.
 */
const GENERATED_LABEL = new Set(["LOOKUP", "LOOKUPS"]);

function pickFieldLabel(dto: FieldPageDto): string {
  const slug = dto.slug ?? "";

  if (dto.type && GENERATED_LABEL.has(dto.type)) {
    const label = dto.attributes?.["label"];
    return typeof label === "string" && label.trim() ? label.trim() : slug;
  }

  return dto.label?.trim() || slug;
}

/**
 * Новая связь между таблицами.
 *
 * Отдельная ручка и отдельное тело: связь — это не поле. POST
 * /v2/fields завёл бы обычную колонку, а связь бэкенд создаёт сам,
 * вместе с колонкой-ссылкой `<таблица_куда>_id` (relation.go:
 * `fieldFrom = data.TableTo + "_id"`). Отсюда же следует, что слаг
 * связи не задаётся руками — его определяет целевая таблица.
 *
 * `table_from` — всегда текущая таблица: связь заводят из неё, и
 * обратное направление означало бы «добавить колонку в чужую таблицу
 * из её меню», чего человек не просил.
 *
 * `relation_table_slug` — та таблица, которая НЕ текущая; у Recursive
 * это она же. Бэкенд по ней ищет строки для вкладки связи в карточке.
 *
 * Подпись пишется дважды — колонкой `label` и в `attributes.label_<язык>`.
 * Так её читает и старая админка, и мы: `label` без языка, attributes —
 * по языкам данных.
 */
export function useCreateRelation(tableSlug: string | undefined) {
  const queryClient = useQueryClient();
  const slug = tableSlug ?? "";

  return useMutation({
    mutationFn: ({ draft, language }: { draft: RelationDraft; language: string }) => {
      const label = draft.label.trim();

      return api.post<unknown>(`/v2/relations/${slug}`, {
        table_from: slug,
        table_to: draft.toSlug,
        type: RELATION_DIRECTION,
        relation_table_slug: draft.toSlug,
        view_fields: draft.viewFieldIds,
        auto_filters: toAutoFiltersBody(draft.autoFilters),
        ...toSelfDefaultBody(draft.selfDefault),
        label,
        attributes: { [`label_${language}`]: label },
      });
    },

    onError: (error) => reportError(error, "common.createFailed"),
    onSuccess: () => {
      toast.success(i18n.t("fieldForm.created", { label: "" }));
      /*
       * Не только схема, но и view: колонку-ссылку бэкенд дописывает
       * в columns всех view таблицы, как и обычное поле. Без этого
       * связь появлялась в списке полей сразу, а колонкой в таблице —
       * только после перезагрузки страницы.
       */
      invalidateSchema(queryClient);
    },
  });
}

/**
 * Правка связи: подпись и поля показа.
 *
 * Тело собирается поверх исходного ответа, как и у поля: PUT
 * перезаписывает строку целиком (relation.go, UPDATE по всем колонкам),
 * и тело, собранное заново, стёрло бы полтора десятка настроек, которых
 * мы не показываем, — auto_filters, cascading, summaries,
 * action_relations, multiple_insert.
 *
 * Целевая таблица не меняется: сменить её — это другая колонка-ссылка
 * в базе и осиротевшие значения во всех строках. Поэтому `table_from`
 * и `table_to` уезжают обратно как пришли, слагами.
 */
export function useUpdateRelation(tableSlug: string | undefined) {
  const queryClient = useQueryClient();
  const slug = tableSlug ?? "";

  return useMutation({
    mutationFn: ({
      relation,
      draft,
      language,
    }: {
      relation: Relation;
      draft: RelationDraft;
      language: string;
    }) => {
      const raw = relation.raw;
      const label = draft.label.trim();
      const attributes = (raw["attributes"] as Record<string, unknown> | undefined) ?? {};

      return api.put<unknown>(`/v2/relations/${slug}`, {
        ...raw,
        table_from: tableOf(raw["table_from"]),
        table_to: tableOf(raw["table_to"]),
        /*
         * Обязателен, а в ответе GET его нет: связь читается развёрнутой,
         * с объектами таблиц, и слаг «той стороны» приходится собирать
         * обратно. Без него PUT отвечает «relation table slug is required».
         */
        relation_table_slug: relation.toSlug,
        // id полей, а не объекты: обратно бэкенд отдаёт их развёрнутыми,
        // но принимает только списком идентификаторов.
        view_fields: draft.viewFieldIds,
        /*
         * Единственная настройка связи, которую мы правим сверх подписи
         * и полей показа. Уезжает всегда, а не только при изменении:
         * UPDATE переписывает колонку безусловно (relation.go:2038),
         * и `...raw` вернул бы прежние пары поверх удалённых.
         */
        auto_filters: toAutoFiltersBody(draft.autoFilters),
        /* Тем же порядком и по той же причине: колонки переписываются
           безусловно, и `...raw` вернул бы прежнее поверх снятого. */
        ...toSelfDefaultBody(draft.selfDefault),
        label,
        attributes: { ...attributes, label, [`label_${language}`]: label },
      });
    },

    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: () => invalidateSchema(queryClient),
  });
}

/**
 * `table_from`/`table_to` приходят объектами, а принимаются слагами.
 * Отдать объект обратно значит записать связь в никуда: бэкенд ждёт
 * строку и молча получает пустую.
 */
function tableOf(value: unknown): string {
  if (typeof value === "string") return value;
  if (typeof value === "object" && value !== null) {
    const slug = (value as { slug?: unknown }).slug;
    return typeof slug === "string" ? slug : "";
  }
  return "";
}

/**
 * Удаление связи.
 *
 * Уходит не только запись о связи: бэкенд роняет и саму колонку-ссылку
 * (`ALTER TABLE ... DROP COLUMN`, pkg/helper/relation.go:1086) вместе
 * со всеми значениями во всех строках, а у Many2Many — и колонку
 * с той стороны. Подтверждение обязан спрашивать вызывающий: отсюда
 * этого уже не видно.
 *
 * Это же единственный способ «перевести связь на другую таблицу»:
 * сменить `table_to` у существующей нельзя — имя колонки выведено
 * из целевой таблицы при создании (relation.go:291), а Update
 * физическую колонку не трогает вовсе. Связь начала бы утверждать
 * одно, а хранить другое.
 */
export function useDeleteRelation(tableSlug: string | undefined) {
  const queryClient = useQueryClient();
  const slug = tableSlug ?? "";

  return useMutation({
    mutationFn: (relation: Relation) =>
      api.delete<unknown>(`/v2/relations/${slug}/${relation.id}`),

    onError: (error) => reportError(error, "common.deleteFailed"),
    onSuccess: () => {
      toast.success(i18n.t("fieldForm.deleted", { label: "" }));
      invalidateSchema(queryClient);
    },
  });
}
