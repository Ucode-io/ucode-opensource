import { useQuery } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { keys } from "@/shared/lib/query-keys";
import type { Field } from "../model/types";
import type { FieldDto } from "./dto";
import { toField } from "./normalize";

/**
 * Поля таблицы вместе с флагом «участвует в поиске».
 *
 * Отдельная ручка — не от хорошей жизни: флаг `is_search` живёт на поле,
 * но GET /v2/fields его не отдаёт (колонки нет в SELECT). Единственная,
 * где он есть, — POST /v1/table-details/{slug}.
 *
 * Оттуда же берётся и сам список полей, а не из схемы: наборы полей
 * у двух ручек не совпадают, и поле, которое есть только в /v2/fields
 * (например ID), выглядело бы вечно выключенным — его флага в ответе
 * просто нет.
 *
 * Запрос идёт, только когда список открывают: флаг нужен одному меню.
 */
export type TableDetailsDto = {
  table_slug?: string;
  /** Конверт клиент снимает один раз, а внутри лежит ещё один `data`. */
  data?: { fields?: (FieldDto & { is_search?: boolean })[] };
};

export type SearchFields = {
  fields: Field[];
  /** id полей, по которым бэкенд ищет. */
  enabled: ReadonlySet<string>;
};

const EMPTY: SearchFields = { fields: [], enabled: new Set() };

export function useSearchFields(tableSlug: string | undefined) {
  const slug = tableSlug ?? "";

  const { data, isFetching } = useQuery({
    queryKey: keys.tables.searchFields(slug),
    // Тело обязательно с ключом data: шлюз пишет в эту карту служебные
    // поля из токена (table.go: objectRequest.Data[...] = ...), и на
    // пустом теле карта nil — ответ 500 без единого слова.
    queryFn: () => api.post<TableDetailsDto>(`/v1/table-details/${slug}`, { data: {} }),
    enabled: Boolean(slug),
    staleTime: 60_000,
    select: toSearchFields,
  });

  return { ...(data ?? EMPTY), isFetching };
}

export function toSearchFields(dto: TableDetailsDto): SearchFields {
  const dtos = dto.data?.fields ?? [];

  return {
    fields: dtos.map(toField),
    enabled: new Set(dtos.filter((field) => field.is_search === true).map((field) => field.id ?? "")),
  };
}
