import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { errorMessage, reportError, toast } from "@/shared/lib/toast";
import type { Labels } from "../model/types";
import { pickLabels } from "./normalize";

/**
 * Действия таблицы — то, что запускают над отмеченными строками.
 *
 * В бэкенде это «автоматизация» (`/v2/collections/{slug}/automation`),
 * в базе — `custom_event`, в старой админке — Actions. Одна и та же
 * сущность под тремя именами; наружу отдаём одно — Action.
 *
 * Действие — это ФУНКЦИЯ проекта плюс подпись и значок: `event_path`
 * хранит id функции, а зовётся она той же ручкой, что и поле-кнопка.
 */

type ActionDto = {
  id?: string;
  table_slug?: string;
  /** id функции проекта, несмотря на имя. */
  event_path?: string;
  label?: string;
  icon?: string;
  /** Куда уйти после успеха. Пусто — остаёмся на месте. */
  url?: string;
  /** Именно `disable`, без «d» на конце: так называется поле в модели. */
  disable?: boolean;
  action_type?: string;
  method?: string;
  path?: string;
  attributes?: Record<string, unknown>;
  /**
   * Право РОЛИ запускать это действие. Приходит по роли из токена
   * (`custom_event.go:253` — JOIN по `role_id`), и `permission` в нём
   * булево, а не 'Yes'/'No', как у прав на таблицу.
   *
   * `id` пустой — записи прав нет вовсе: `permission` в этом случае
   * не «нельзя», а COALESCE до false. См. isAllowed.
   */
  action_permission?: { id?: string; permission?: boolean };
};

type ActionsResponse = { custom_events?: ActionDto[] };

export type Action = {
  id: string;
  /** Подпись без языка — колонка `label`. */
  label: string;
  /** Подписи по языкам ДАННЫХ: attributes.label_<код>. */
  labels: Labels;
  /** id функции проекта. Пусто — действию нечего звать. */
  functionId: string;
  icon: string;
  url: string;
  /** Выключенное действие в списке не показывается. */
  disabled: boolean;
  /** Роли разрешено его запускать. См. isAllowed. */
  allowed: boolean;
  /** HTTP | after | before — когда функция срабатывает. */
  actionType: string;
  /** Над какой операцией: GETLIST, UPDATE, CREATE… */
  method: string;
  /** Перезапросить строки после успеха. */
  refresh: boolean;
  raw: Record<string, unknown>;
};

/** Когда действие срабатывает. Значения из старой админки, они же в базе. */
export const ACTION_TYPES = ["HTTP", "before", "after"] as const;

/** Над какой операцией срабатывает действие. */
export const ACTION_METHODS = [
  "GETLIST",
  "CREATE",
  "UPDATE",
  "DELETE",
  "EXCEL_IMPORT",
  "MULTIPLE_UPDATE",
  "APPEND_MANY2MANY",
  "DELETE_MANY2MANY",
] as const;

export function useActions(tableSlug: string | undefined, enabled = true) {
  const slug = tableSlug ?? "";

  const query = useQuery({
    queryKey: keys.actions.byTable(slug),
    queryFn: () =>
      api.get<ActionsResponse>(`/v2/collections/${slug}/automation`, {
        params: { table_slug: slug },
      }),
    enabled: Boolean(slug) && enabled,
    // Действия заводит админ, а не пользователь при работе со строками.
    staleTime: 5 * 60_000,
    select: (data) => (data.custom_events ?? []).filter((dto) => dto.id).map(toAction),
  });

  return {
    actions: query.data ?? [],
    isLoading: query.isLoading,
    /** Причина отказа словами. null — всё в порядке. */
    error: errorMessage(query.error, "table.loadFailed"),
  };
}

/** Черновик действия: то, что правится в форме. */
export type ActionDraft = {
  labels: Labels;
  functionId: string;
  icon: string;
  url: string;
  actionType: string;
  method: string;
  refresh: boolean;
  disabled: boolean;
};

export const EMPTY_ACTION_DRAFT: ActionDraft = {
  labels: {},
  functionId: "",
  icon: "",
  url: "",
  actionType: "HTTP",
  method: "GETLIST",
  refresh: false,
  disabled: false,
};

export function toDraft(action: Action): ActionDraft {
  return {
    labels: action.labels,
    functionId: action.functionId,
    icon: action.icon,
    url: action.url,
    actionType: action.actionType,
    method: action.method,
    refresh: action.refresh,
    disabled: action.disabled,
  };
}

export function useCreateAction(tableSlug: string | undefined) {
  const queryClient = useQueryClient();
  const slug = tableSlug ?? "";

  return useMutation({
    mutationFn: (draft: ActionDraft) =>
      api.post<unknown>(`/v2/collections/${slug}/automation`, toBody(draft, slug)),
    onError: (error) => reportError(error, "common.createFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("actions.created"));
      await queryClient.invalidateQueries({ queryKey: keys.actions.byTable(slug) });
    },
  });
}

/**
 * Правка действия. Тело — поверх исходного ответа: UPDATE пишет строку
 * целиком, и собранное заново тело стёрло бы `attributes.additional_parameters`
 * — набор значений, которые уезжают в функцию вместе со строками.
 * Мы их не показываем (их смысл задаётся графом связей таблицы),
 * но и терять их нельзя.
 */
export function useUpdateAction(tableSlug: string | undefined) {
  const queryClient = useQueryClient();
  const slug = tableSlug ?? "";

  return useMutation({
    mutationFn: ({ action, draft }: { action: Action; draft: ActionDraft }) => {
      const body = toBody(draft, slug);
      const previous = (action.raw["attributes"] as Record<string, unknown> | undefined) ?? {};

      return api.put<unknown>(`/v2/collections/${slug}/automation`, {
        ...action.raw,
        ...body,
        id: action.id,
        // Attributes дописываются, а не заменяются: в них лежит
        // additional_parameters, которого мы не показываем.
        attributes: { ...previous, ...(body["attributes"] as Record<string, unknown>) },
      });
    },
    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("actions.saved"));
      await queryClient.invalidateQueries({ queryKey: keys.actions.byTable(slug) });
    },
  });
}

export function useDeleteAction(tableSlug: string | undefined) {
  const queryClient = useQueryClient();
  const slug = tableSlug ?? "";

  return useMutation({
    mutationFn: (action: Action) =>
      api.delete<unknown>(`/v2/collections/${slug}/automation/${action.id}`),
    onError: (error) => reportError(error, "common.deleteFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("actions.deleted"));
      await queryClient.invalidateQueries({ queryKey: keys.actions.byTable(slug) });
    },
  });
}

/**
 * Запуск действия над отмеченными строками.
 *
 * Ручка та же, что у поля-кнопки: действие — это функция проекта,
 * и зовётся она по id (`event_path`). Отличие одно — строк много.
 *
 * Ответ может содержать адрес: функция сообщает, куда идти дальше.
 * Особые значения `reload` и `reloadRelations` из старой админки
 * не переносятся — там они означали перезагрузку страницы целиком;
 * у нас список перезапрашивается сам.
 */
export function useRunAction(tableSlug: string | undefined) {
  const queryClient = useQueryClient();
  const slug = tableSlug ?? "";

  return useMutation({
    mutationFn: ({ action, guids }: { action: Action; guids: string[] }) =>
      api.post<{ url?: string; status?: string; message?: string }>("/v1/invoke_function", {
        function_id: action.functionId,
        table_slug: slug,
        object_ids: guids,
      }),

    onError: (error) => reportError(error, "actions.failed"),
    onSuccess: async (data, { action }) => {
      /*
       * Отказ приезжает со статусом 200 и полем status: "error" — так
       * отвечает сама функция, а не шлюз. Без этой проверки провал
       * функции выглядел бы успехом.
       */
      if (data?.status === "error") {
        toast.error(data.message || i18n.t("actions.failed"));
        return;
      }

      toast.success(i18n.t("actions.done"));
      await queryClient.invalidateQueries({ queryKey: keys.items.table(slug) });

      // Адрес из ответа важнее записанного в настройках: функция знает,
      // какую именно страницу она только что подготовила.
      const url = typeof data?.url === "string" && data.url ? data.url : action.url;
      if (url) window.open(url, "_blank", "noopener,noreferrer");
    },
  });
}

/**
 * Черновик → тело запроса.
 *
 * Подпись пишется и колонкой `label`, и ключами по языкам данных —
 * как у полей, view и таблиц. Колонка одна на все языки, поэтому
 * в неё идёт первое непустое имя.
 */
function toBody(draft: ActionDraft, tableSlug: string): Record<string, unknown> {
  const labels = Object.fromEntries(
    Object.entries(draft.labels).map(([code, value]) => [`label_${code}`, value.trim()]),
  );

  return {
    table_slug: tableSlug,
    event_path: draft.functionId,
    label: firstNamed(draft.labels),
    icon: draft.icon.trim(),
    url: draft.url.trim(),
    // Поле называется `disable`, без «d»: старая админка писала `disabled`,
    // и выключить действие через её форму было нельзя.
    disable: draft.disabled,
    action_type: draft.actionType,
    method: draft.method,
    attributes: { ...labels, use_refresh: draft.refresh },
  };
}

function firstNamed(labels: Labels): string {
  for (const value of Object.values(labels)) {
    if (value.trim()) return value.trim();
  }
  return "";
}

/**
 * Разрешено ли роли запускать действие.
 *
 * Запрет строгий, разрешение по умолчанию — как и у прав на поля, но
 * различить их здесь можно только по `id`. Запрос списка действий
 * подставляет право через `COALESCE(ac.permission, false)`
 * (custom_event.go:239), то есть у роли БЕЗ записи прав приходит ровно
 * то же `false`, что и у явного запрета. Отличает их пустой `id`:
 * записи нет — значит, никто ничего не запрещал.
 *
 * Разница не теоретическая. Записи прав заводятся один раз, когда
 * действие создают, и только для ролей, которые к тому моменту уже
 * есть (custom_event.go:105); роль, заведённая позже, записи не получает
 * никогда, а правка прав — это UPDATE без вставки (permission.go:1690).
 * Считай мы такую роль запрещённой — она потеряла бы все действия
 * таблицы, и вернуть их через настройки было бы нечем.
 */
function isAllowed(permission: ActionDto["action_permission"]): boolean {
  if (!permission?.id) return true;
  return permission.permission === true;
}

export function toAction(dto: ActionDto): Action {
  const attributes = dto.attributes ?? {};

  return {
    id: dto.id ?? "",
    label: dto.label?.trim() ?? "",
    labels: pickLabels(attributes),
    functionId: dto.event_path ?? "",
    icon: dto.icon ?? "",
    url: dto.url ?? "",
    disabled: dto.disable === true,
    allowed: isAllowed(dto.action_permission),
    actionType: dto.action_type ?? "",
    method: dto.method ?? "",
    refresh: attributes["use_refresh"] === true,
    raw: { ...dto },
  };
}
