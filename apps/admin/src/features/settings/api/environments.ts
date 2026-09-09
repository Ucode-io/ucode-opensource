import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { reportError, toast } from "@/shared/lib/toast";

/**
 * Окружения проекта: завести, переименовать, удалить.
 *
 * Списка здесь нет намеренно — его грузит features/workspace для
 * переключателя в шапке, и второй запрос за тем же адресом означал бы
 * два списка окружений, расходящихся после первой же правки. Правки
 * протухают ЕГО ключ (`keys.workspace.environments`).
 *
 * Само переключение окружения тоже там: это переход с ротацией токенов,
 * а не настройка (docs/adr/0001).
 */
const ENVIRONMENTS = "/v1/environment";

export type EnvironmentDraft = {
  name: string;
  /** HEX из палитры чипов: цвет уезжает в данные, а рисуется токеном. */
  color: string;
  description: string;
};

/**
 * Новое окружение.
 *
 * Проект и компания уезжают в теле: из токена шлюз достаёт только роль,
 * пользователя и тип клиента (`environment.go:79`), а `project_id`
 * и `company_id` читает из запроса. Компанию берём из карточки проекта —
 * другого места, где она есть, у нас нет.
 *
 * Заведение окружения — это ещё и заведение его ресурсов на стороне
 * компании, поэтому ответ приходит не мгновенно; кнопка ждёт его,
 * а не закрывается сразу.
 */
export function useCreateEnvironment(companyId: string) {
  const queryClient = useQueryClient();
  const projectId = useSession().getProjectId() ?? "";

  return useMutation({
    mutationFn: (draft: EnvironmentDraft) =>
      api.post<unknown>(ENVIRONMENTS, {
        project_id: projectId,
        company_id: companyId,
        name: draft.name.trim(),
        display_color: draft.color,
        description: draft.description.trim(),
      }),
    onError: (error) => reportError(error, "common.createFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("environments.created"));
      await queryClient.invalidateQueries({
        queryKey: keys.workspace.environments(projectId),
      });
    },
  });
}

/**
 * Правка окружения. Шлюз собирает объект сам из пяти полей
 * (`environment.go:220`) — остальное в теле он не читает, поэтому
 * и посылать его незачем.
 */
export function useUpdateEnvironment() {
  const queryClient = useQueryClient();
  const projectId = useSession().getProjectId() ?? "";

  return useMutation({
    mutationFn: ({ id, draft }: { id: string; draft: EnvironmentDraft }) =>
      api.put<unknown>(ENVIRONMENTS, {
        id,
        project_id: projectId,
        name: draft.name.trim(),
        display_color: draft.color,
        description: draft.description.trim(),
      }),
    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("settings.saved"));
      await queryClient.invalidateQueries({
        queryKey: keys.workspace.environments(projectId),
      });
    },
  });
}

/**
 * Удаление окружения. Вместе с ним уходят ЕГО данные: у таблицы одна
 * схема на проект, но строки свои в каждом окружении (см. CONTEXT).
 * Поэтому подтверждение с именем, а не просто «удалить?».
 */
export function useDeleteEnvironment() {
  const queryClient = useQueryClient();
  const projectId = useSession().getProjectId() ?? "";

  return useMutation({
    mutationFn: (id: string) => api.delete<unknown>(`${ENVIRONMENTS}/${id}`),
    onError: (error) => reportError(error, "common.deleteFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("environments.deleted"));
      await queryClient.invalidateQueries({
        queryKey: keys.workspace.environments(projectId),
      });
    },
  });
}
