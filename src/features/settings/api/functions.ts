import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { errorMessage, reportError, toast } from "@/shared/lib/toast";

/**
 * Функции проекта: код, который выполняется на стороне ucode.
 *
 * Живут они в `ucode_go_function_service`, а шлюз проксирует его группу
 * `/v2/function` как есть (`api.go:842`, `MakeProxy`). Отсюда и адреса.
 *
 * **Кода отсюда не записать, и это решение бэкенда, а не наше.** Ручка,
 * которая пишет файлы функции обратно, существует только у
 * микрофронтенда (`/v2/functions/micro-frontend/push-changes`).
 * У функции есть чтение — `GET /v2/function/:id/codebase`, сервис
 * скачивает архив ветки `u-gen` своим токеном GitLab
 * (`function_service/pkg/gitlab/gitlab.go:1131`). Поэтому исходники
 * у нас показываются, а не правятся: редактор, который не может
 * сохранить, — обещание, которого не выполнить.
 */
export type ProjectFunction = {
  id: string;
  name: string;
  /** Путь запуска. Собирает его бэкенд из имени проекта, руками не задать. */
  path: string;
  description: string;
  /** FUNCTION | KNATIVE | WORKFLOW. */
  type: string;
  branch: string;
  /** Статус сборки в CI. Пусто — сборки ещё не было. */
  pipelineStatus: string;
  /** Что сказал CI, когда сборка упала. */
  errorMessage: string;
  /**
   * Ответ сервера как есть. `PUT` перезаписывает строку целиком
   * (`object_builder/storage/postgres/function.go:431` — SET по двадцати
   * колонкам сразу), и всё, чего в теле не было, обнуляется: адрес,
   * пароль, ссылка на GitLab, статус сборки. Поэтому правка уезжает
   * поверх исходного ответа, как у полей таблицы.
   */
  raw: Record<string, unknown>;
};

type FunctionDto = {
  id?: string;
  name?: string;
  path?: string;
  description?: string;
  type?: string;
  branch?: string;
  pipeline_status?: string;
  error_message?: string;
};

type ListDto = { functions?: FunctionDto[] | null; count?: number };

/**
 * Типы, которые бэкенд принимает при создании. Остальное он отклоняет
 * словами «not supported function type» (`function_service/api/handlers/
 * function.go:204`).
 *
 * FUNCTION и KNATIVE заводят себе репозиторий в GitLab — форк шаблона,
 * из которого потом собирается образ. WORKFLOW не заводит ничего:
 * ветка `case config.WORKFLOW` в том же switch пустая, и такая функция
 * остаётся строкой в базе без репозитория. Значит и исходников у неё
 * не будет.
 */
export const FUNCTION_TYPES = ["FUNCTION", "KNATIVE", "WORKFLOW"] as const;

/** Строк на странице раздела. */
export const FUNCTIONS_PAGE = 20;

function toFunction(dto: FunctionDto): ProjectFunction {
  return {
    id: dto.id ?? "",
    name: dto.name?.trim() ?? "",
    path: dto.path ?? "",
    description: dto.description ?? "",
    type: dto.type ?? "",
    branch: dto.branch ?? "",
    pipelineStatus: dto.pipeline_status ?? "",
    errorMessage: dto.error_message ?? "",
    raw: { ...dto },
  };
}

export function useProjectFunctions(search: string, page: number) {
  const envId = useSession().getEnvironmentId() ?? "";
  const query = useQuery({
    queryKey: keys.functions.page(envId, search, page),
    queryFn: () =>
      api.get<ListDto>("/v2/function", {
        params: {
          search,
          limit: FUNCTIONS_PAGE,
          offset: (page - 1) * FUNCTIONS_PAGE,
        },
      }),
  });

  return {
    functions: (query.data?.functions ?? []).filter((dto) => dto.id).map(toFunction),
    count: query.data?.count ?? 0,
    isLoading: query.isLoading,
    error: errorMessage(query.error, "functions.loadFailed"),
  };
}

/**
 * Исходники функции: весь репозиторий одним ответом,
 * `{files: [{path, content}]}`.
 *
 * Запрос уходит только у открытой карточки: это архив ветки целиком,
 * и держать его для списка незачем. Заодно у этого чтения есть
 * побочное действие на стороне GitLab — ветку `u-gen` сервис СОЗДАЁТ,
 * если её нет (`ensureUGenBranch`, там же). Поэтому не префетчим.
 */
export function useFunctionCodebase(id: string) {
  const envId = useSession().getEnvironmentId() ?? "";
  const query = useQuery({
    queryKey: keys.functions.codebase(envId, id),
    queryFn: () => api.get<{ files?: { path?: string; content?: string }[] }>(
      `/v2/function/${id}/codebase`,
    ),
    enabled: Boolean(id),
    // Код меняют не здесь: он приезжает из репозитория.
    staleTime: 5 * 60_000,
  });

  return {
    files: (query.data?.files ?? [])
      .filter((file): file is { path: string; content?: string } => Boolean(file.path))
      .map((file) => ({ path: file.path, content: file.content ?? "" })),
    isLoading: query.isLoading,
    error: errorMessage(query.error, "functions.codeFailed"),
  };
}

export type FunctionDraft = { name: string; path: string; description: string; type: string };

export const EMPTY_FUNCTION_DRAFT: FunctionDraft = {
  name: "",
  path: "",
  description: "",
  type: "FUNCTION",
};

function useInvalidate() {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: keys.functions.all });
}

/**
 * Завести функцию.
 *
 * Тело — четыре поля, и больше отправлять нечего: остальное, что
 * принимает модель запроса (`branch`, `repo_name`, `framework_type`),
 * обработчик до сервиса не доносит вовсе — он собирает `CreateFunctionRequest`
 * сам, а ветку и репозиторий берёт из форка, который только что создал
 * (`function_service/api/handlers/function.go:206–235`).
 *
 * Путь тоже не окончательный: к нему спереди приклеивается имя проекта,
 * а всё, кроме латиницы, цифр и `_-.`, вырезается. Показываем это
 * подсказкой, а не переписываем ввод: правило чужое и может измениться.
 */
export function useCreateFunction() {
  const invalidate = useInvalidate();

  return useMutation({
    mutationFn: (draft: FunctionDraft) =>
      api.post<unknown>("/v2/function", {
        name: draft.name.trim(),
        path: draft.path.trim(),
        description: draft.description.trim(),
        type: draft.type,
      }),
    onError: (error) => reportError(error, "common.createFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("functions.created"));
      await invalidate();
    },
  });
}

export function useUpdateFunction() {
  const invalidate = useInvalidate();

  return useMutation({
    mutationFn: ({ item, draft }: { item: ProjectFunction; draft: FunctionDraft }) =>
      api.put<unknown>("/v2/function", {
        ...item.raw,
        id: item.id,
        name: draft.name.trim(),
        description: draft.description.trim(),
        /*
         * Путь и тип не правим. Путь — это адрес развёрнутой функции
         * и имя репозитория: строка в базе поменялась бы, а запущенное
         * осталось бы под прежним именем. Тип решает, из какого шаблона
         * форкнут репозиторий, и задним числом его не переиграть.
         */
      }),
    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("functions.saved"));
      await invalidate();
    },
  });
}

export function useDeleteFunction() {
  const invalidate = useInvalidate();

  return useMutation({
    mutationFn: (item: ProjectFunction) => api.delete<unknown>(`/v2/function/${item.id}`),
    onError: (error) => reportError(error, "common.deleteFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("functions.deleted"));
      await invalidate();
    },
  });
}

/**
 * Запустить функцию и показать, что она ответила.
 *
 * Ручка не та, которой действия таблицы зовут функцию по строкам:
 * `POST /v1/invoke_function` отвечает пустотой независимо от результата
 * (см. docs/backend-notes.md, «Функции»). Здесь — `/v2/functions/:id/run`,
 * которая ходит в саму функцию и отдаёт её ответ наружу
 * (`api/handlers/v1/function_v2.go:229`), в том числе `message` из
 * ошибки. Ради этого запуск и имеет смысл: он показывает результат.
 */
export function useRunFunction() {
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: unknown }) =>
      api.post<unknown>(`/v2/functions/${id}/run`, body),
    onError: (error) => reportError(error, "functions.runFailed"),
  });
}
