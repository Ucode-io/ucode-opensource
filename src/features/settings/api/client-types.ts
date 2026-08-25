import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, authApi } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { reportError, toast } from "@/shared/lib/toast";

/**
 * Типы клиентов — аудитории проекта.
 *
 * Тип клиента отвечает на вопрос «кто входит»: у него своя таблица
 * входа, свои роли и свои люди. Роль без типа никуда не пускает,
 * а список людей приходит по одному типу за раз — поэтому и вкладки
 * в разделе «Пользователи», и обязательный выбор при создании роли.
 *
 * Живёт на сервере авторизации, но хранится не там: `/v2/client-type`
 * пишет строку в системную таблицу `client_type` проекта через
 * object-builder. Отсюда конверт `{data: {response}}` — тот же, что
 * у строк обычной таблицы.
 *
 * Таблица входа необязательна: если её не выбрать, бэкенд заведёт свою
 * (`<имя>_users`) вместе с типом (`client_service_v2.go:72`).
 */
const CLIENT_TYPES = "/v2/client-type";

type ClientTypeDto = {
  guid?: string;
  name?: string;
  table_slug?: string;
  /** Сколько сессий разом держит один человек этого типа. По умолчанию 50. */
  session_limit?: number;
  self_register?: boolean;
  self_recover?: boolean;
  default_page?: string;
  /** UNDECIDED | PHONE | EMAIL. Не показываем — см. useUpdateClientType. */
  confirm_by?: string;
};

type ListDto = { data?: { response?: ClientTypeDto[] } };
type SingleDto = { data?: { response?: ClientTypeDto } };

export type ClientType = {
  id: string;
  name: string;
  tableSlug: string;
  sessionLimit: number;
  selfRegister: boolean;
  selfRecover: boolean;
  defaultPage: string;
  confirmBy: string;
};

export function toClientType(dto: ClientTypeDto): ClientType {
  return {
    id: dto.guid ?? "",
    name: dto.name?.trim() || dto.guid || "",
    tableSlug: dto.table_slug?.trim() ?? "",
    // 0 в ответе — это «колонки не было», а не «ноль сессий»: колонку
    // добавили миграцией 000023 со значением по умолчанию 50.
    sessionLimit: dto.session_limit || DEFAULT_SESSION_LIMIT,
    selfRegister: dto.self_register === true,
    selfRecover: dto.self_recover === true,
    defaultPage: dto.default_page?.trim() ?? "",
    confirmBy: dto.confirm_by?.trim() || "UNDECIDED",
  };
}

export const DEFAULT_SESSION_LIMIT = 50;

export function useClientTypes(enabled = true) {
  const projectId = useSession().getProjectId() ?? "";

  const query = useQuery({
    queryKey: keys.settings.clientTypes(projectId),
    queryFn: () =>
      authApi.get<ListDto>(CLIENT_TYPES, {
        params: { "project-id": projectId, limit: 50, offset: 0 },
      }),
    enabled: enabled && Boolean(projectId),
    staleTime: 5 * 60_000,
    select: (data): ClientType[] =>
      (data.data?.response ?? []).filter((dto) => dto.guid).map(toClientType),
  });

  return { clientTypes: query.data ?? NO_CLIENT_TYPES, isLoading: query.isLoading };
}

const NO_CLIENT_TYPES: ClientType[] = [];

/**
 * Один тип целиком — тем, кто его правит.
 *
 * Форма читает ЕГО, а не строку списка: на mongo-проектах список
 * отдаётся урезанным (`GetListSlim`, `client_service_v2.go:306`), и
 * `session_limit` в нём может не приехать. Сохранение поверх такого
 * ответа записало бы в базу ноль вместо настоящего предела.
 */
export function useClientType(id: string) {
  const projectId = useSession().getProjectId() ?? "";

  const query = useQuery({
    queryKey: keys.settings.clientType(projectId, id),
    queryFn: () =>
      authApi.get<SingleDto>(`${CLIENT_TYPES}/${id}`, {
        params: { "project-id": projectId },
      }),
    enabled: Boolean(projectId && id),
    select: (data) => (data.data?.response ? toClientType(data.data.response) : undefined),
  });

  return { clientType: query.data, isLoading: query.isLoading };
}

/**
 * Таблицы входа проекта — из чего выбирают.
 *
 * Свой запрос, а не список таблиц из features/table: тот постраничный
 * и с поиском, потому что таблиц в проекте сотни. Таблиц входа —
 * единицы: их помечает флажок в настройках таблицы, и отбирает их сам
 * шлюз (`collection.go:272`).
 */
export type LoginTable = { slug: string; label: string };

export function useLoginTables(enabled = true) {
  const projectId = useSession().getProjectId() ?? "";

  const query = useQuery({
    queryKey: keys.settings.loginTables(projectId),
    queryFn: () =>
      api.get<{ tables?: { slug?: string; label?: string }[] }>("/v2/collections", {
        params: { is_login_table: true, limit: 100, offset: 0 },
      }),
    enabled: enabled && Boolean(projectId),
    staleTime: 5 * 60_000,
    select: (data): LoginTable[] =>
      (data.tables ?? [])
        .filter((dto) => dto.slug)
        .map((dto) => ({ slug: dto.slug ?? "", label: dto.label?.trim() || dto.slug || "" })),
  });

  return { loginTables: query.data ?? NO_LOGIN_TABLES, isLoading: query.isLoading };
}

const NO_LOGIN_TABLES: LoginTable[] = [];

export type ClientTypeDraft = {
  name: string;
  /** Пусто при создании — бэкенд заведёт таблицу входа сам. */
  tableSlug: string;
  sessionLimit: number;
  selfRegister: boolean;
  /** Только при создании: правка его теряет — см. useUpdateClientType. */
  selfRecover: boolean;
  /** Только при правке: создание его теряет — см. useCreateClientType. */
  defaultPage: string;
};

/**
 * Новый тип клиента.
 *
 * `default_page` не посылаем: до базы он и не доедет — шлюз собирает
 * тело для object-builder из девяти полей, и адреса среди них нет
 * (`client_service_v2.go:43`). Задать его можно правкой, где он есть.
 *
 * Второй запрос — обход бага, а не шаг настройки. Когда таблицу входа
 * не выбрали, бэкенд заводит её сам, но записывает в тип ДРУГОЙ слаг:
 * таблицу называет `strings.ReplaceAll(name, " ", "_")`, а в строку
 * кладёт имя без замены пробела (`client_service_v2.go:72` и `:96`).
 * У типа «Delivery Drivers» получается ссылка на несуществующую
 * `delivery drivers_users`, и завести человека в него уже нельзя.
 * Чиним сразу — иначе это обнаружится на первом же пользователе.
 */
export function useCreateClientType() {
  const queryClient = useQueryClient();
  const projectId = useSession().getProjectId() ?? "";

  return useMutation({
    mutationFn: async (draft: ClientTypeDraft) => {
      const name = draft.name.trim();

      const created = await authApi.post<{ data?: { guid?: string } }>(
        CLIENT_TYPES,
        {
          name,
          table_slug: draft.tableSlug,
          session_limit: draft.sessionLimit,
          self_register: draft.selfRegister,
          self_recover: draft.selfRecover,
        },
        { params: { "project-id": projectId } },
      );

      const guid = created.data?.guid ?? "";
      if (draft.tableSlug || !name.includes(" ") || !guid) return;

      await authApi.put<unknown>(
        CLIENT_TYPES,
        {
          guid,
          name,
          table_slug: `${name.toLowerCase().replaceAll(" ", "_")}_users`,
          session_limit: draft.sessionLimit,
          self_register: draft.selfRegister,
          default_page: "",
          confirm_by: 0,
        },
        { params: { "project-id": projectId } },
      );
    },
    onError: (error) => reportError(error, "common.createFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("clientTypes.created"));
      await queryClient.invalidateQueries({ queryKey: keys.settings.clientTypes(projectId) });
    },
  });
}

/**
 * Правка типа клиента.
 *
 * `confirm_by` уезжает обратно тем же, каким приехал, и числом:
 * шлюз читает его в enum-поле proto (`V2UpdateClientTypeRequest`)
 * и всегда пишет в базу — не послать его значит перезаписать
 * сохранённое значение нулём («UNDECIDED»). Строку encoding/json
 * в это поле не разберёт, отсюда карта.
 *
 * `self_recover` не посылаем вовсе: шлюз кладёт его в тело под именем
 * `self_recorder` (`client_service_v2.go:364`), колонки с таким именем
 * нет, и правка молча ничего не меняет. Прежнее значение при этом
 * остаётся целым — см. docs/backend-notes.md.
 */
const CONFIRM_BY: Record<string, number> = { UNDECIDED: 0, PHONE: 1, EMAIL: 2 };

export function useUpdateClientType() {
  const queryClient = useQueryClient();
  const projectId = useSession().getProjectId() ?? "";

  return useMutation({
    mutationFn: ({ clientType, draft }: { clientType: ClientType; draft: ClientTypeDraft }) =>
      authApi.put<unknown>(
        CLIENT_TYPES,
        {
          guid: clientType.id,
          name: draft.name.trim(),
          table_slug: draft.tableSlug,
          session_limit: draft.sessionLimit,
          self_register: draft.selfRegister,
          default_page: draft.defaultPage.trim(),
          confirm_by: CONFIRM_BY[clientType.confirmBy] ?? 0,
        },
        { params: { "project-id": projectId } },
      ),
    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("settings.saved"));
      await queryClient.invalidateQueries({ queryKey: keys.settings.clientTypes(projectId) });
    },
  });
}

/**
 * Удаление типа клиента. Роли и люди этого типа остаются в базе, но
 * входить им становится некуда — поэтому подтверждение с именем.
 */
export function useDeleteClientType() {
  const queryClient = useQueryClient();
  const projectId = useSession().getProjectId() ?? "";

  return useMutation({
    mutationFn: (id: string) =>
      authApi.delete<unknown>(`${CLIENT_TYPES}/${id}`, {
        params: { "project-id": projectId },
      }),
    onError: (error) => reportError(error, "common.deleteFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("clientTypes.deleted"));
      await queryClient.invalidateQueries({ queryKey: keys.settings.clientTypes(projectId) });
    },
  });
}
