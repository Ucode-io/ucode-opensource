import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { errorMessage, reportError, toast } from "@/shared/lib/toast";

/**
 * Управление микрофронтендами: список, заведение, версии, публикация.
 *
 * Отдельно от `api/microfrontend`, где живёт ЧТЕНИЕ ради показа пунктом
 * меню: тому экрану нужен один адрес сборки, а этому — номер
 * репозитория, ветка, статус сборки и история. Запрос списка при этом
 * один и тот же, ключ общий: два `select` над одним ответом дешевле
 * второго похода в сеть.
 *
 * **Чего здесь нет и почему.**
 *
 * ПРАВКИ записи нет. `PUT /v2/functions/micro-frontend` сломан в обе
 * стороны. На PostgreSQL он не работает вовсе: в ветке
 * `case pb.ResourceType_POSTGRESQL` объявлена новая переменная с тем же
 * именем, и в сервис уходит ПУСТАЯ структура вместо заполненной
 * (`function_service/api/handlers/microfrontend.go:564`) — без `id`
 * и без `project_id`. На MongoDB он работает и стирает: обработчик
 * собирает объект из шести полей, а `Update` в object_builder пишет
 * `SET` по двадцати колонкам сразу, то есть `url`, `environment_id`,
 * `repo_id`-соседи и статус сборки обнуляются. Кнопки «переименовать»
 * поэтому нет: она либо ничего не сделает, либо сломает микрофронтенд.
 * См. docs/backend-notes.md.
 *
 * РЕДАКТОРА ФАЙЛОВ нет. Ручки чтения и записи файлов ветки `u-gen`
 * в сервисе есть (`GET …/micro-frontend/files`,
 * `PUT …/micro-frontend/push-changes`), но шлюз их НЕ проксирует:
 * в его группе `proxyFunctions` перечислены одиннадцать маршрутов,
 * и этих двух среди них нет (`api/api.go:853–868`). Из админки они
 * недостижимы. Показать файлы всё же можно — снимком версии
 * (`files-at-commit`), и это чтение.
 */
export type Microfrontend = {
  id: string;
  name: string;
  /** Голый хост сборки, без схемы. */
  url: string;
  description: string;
  /** Номер проекта в GitLab. Им заведуют версии и публикация. */
  repoId: string;
  branch: string;
  pipelineStatus: string;
  errorMessage: string;
  /**
   * Проект MCP, из которого микрофронтенд сгенерирован.
   *
   * Пусто у всех, кого завели руками: `CreateMicroFrontEnd` этого поля
   * не заполняет (`microfrontend.go:232`), его ставит только публикация
   * из генератора. А публикация в master требует и его, и номер
   * репозитория (`microfrontend.go:969`), поэтому у заведённых руками
   * кнопки публикации не будет — она вернула бы 400.
   */
  mcpProjectId: string;
};

type MicrofrontendDto = {
  id?: string;
  name?: string;
  url?: string;
  description?: string;
  repo_id?: string;
  branch?: string;
  pipeline_status?: string;
  error_message?: string;
  mcp_project_id?: string;
};

const MICROFRONTENDS = "/v2/functions/micro-frontend";

function toMicrofrontend(dto: MicrofrontendDto): Microfrontend {
  return {
    id: dto.id ?? "",
    name: dto.name?.trim() ?? "",
    url: dto.url?.trim() ?? "",
    description: dto.description ?? "",
    repoId: dto.repo_id ?? "",
    branch: dto.branch ?? "",
    pipelineStatus: dto.pipeline_status ?? "",
    errorMessage: dto.error_message ?? "",
    mcpProjectId: dto.mcp_project_id ?? "",
  };
}

/** Список для управления: те же данные, что у выбора, но целиком. */
export function useManagedMicrofrontends() {
  const envId = useSession().getEnvironmentId() ?? "";

  const query = useQuery({
    queryKey: [...keys.microfrontends.all, envId, "list"] as const,
    queryFn: () => api.get<{ functions?: MicrofrontendDto[] | null }>(MICROFRONTENDS),
    select: (dto): Microfrontend[] =>
      (dto.functions ?? []).filter((item) => item.id).map(toMicrofrontend),
  });

  return {
    items: query.data ?? [],
    isLoading: query.isLoading,
    error: errorMessage(query.error, "microfrontends.loadFailed"),
  };
}

function useInvalidate() {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: keys.microfrontends.all });
}

/**
 * Завести микрофронтенд.
 *
 * Путь проверяется бэкендом строго — `[a-z]`, цифры и дефис
 * (`IsValidFunctionName`), иначе отказ словами. По этому пути он
 * форкает шаблон в GitLab и записывает номер получившегося
 * репозитория; адрес сборки тоже придумывает он.
 */
export function useCreateMicrofrontend() {
  const invalidate = useInvalidate();

  return useMutation({
    mutationFn: ({ name, path, description }: { name: string; path: string; description: string }) =>
      api.post<unknown>(MICROFRONTENDS, {
        name: name.trim(),
        path: path.trim(),
        description: description.trim(),
      }),
    onError: (error) => reportError(error, "common.createFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("microfrontends.created"));
      await invalidate();
    },
  });
}

export function useDeleteMicrofrontend() {
  const invalidate = useInvalidate();

  return useMutation({
    mutationFn: (item: Microfrontend) => api.delete<unknown>(`${MICROFRONTENDS}/${item.id}`),
    onError: (error) => reportError(error, "common.deleteFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("microfrontends.deleted"));
      await invalidate();
    },
  });
}

export type Commit = {
  sha: string;
  shortSha: string;
  title: string;
  author: string;
  date: string;
  webUrl: string;
};

type CommitDto = {
  id?: string;
  short_id?: string;
  title?: string;
  author_name?: string;
  committed_date?: string;
  web_url?: string;
};

/**
 * Версии: коммиты ветки `u-gen`, и только те, что сделаны токеном
 * самого сервиса — чужие ручные коммиты он отсеивает по почте автора
 * (`microfrontend_commits.go:88`). То есть это история изменений
 * ИЗ АДМИНКИ, а не история репозитория.
 */
export function useMicrofrontendCommits(repoId: string) {
  const query = useQuery({
    queryKey: keys.microfrontends.commits(repoId),
    queryFn: () =>
      api.get<CommitDto[]>(`${MICROFRONTENDS}/commits`, { params: { repo_id: repoId, limit: 50 } }),
    enabled: Boolean(repoId),
  });

  return {
    commits: (query.data ?? []).map(
      (dto): Commit => ({
        sha: dto.id ?? "",
        shortSha: dto.short_id ?? "",
        title: dto.title?.trim() ?? "",
        author: dto.author_name ?? "",
        date: dto.committed_date ?? "",
        webUrl: dto.web_url ?? "",
      }),
    ),
    isLoading: query.isLoading,
    error: errorMessage(query.error, "microfrontends.commitsFailed"),
  };
}

/**
 * Снимок версии: все файлы репозитория на выбранном коммите.
 *
 * Ответ — массив `{file_path, content}` без обёртки. Запрашивается
 * только у раскрытой версии: сервис читает файлы по одному, каждый
 * своим запросом в GitLab (`microfrontend_commits.go:159`), и на
 * большом проекте это долго.
 */
export function useFilesAtCommit(repoId: string, sha: string) {
  const query = useQuery({
    queryKey: keys.microfrontends.filesAt(repoId, sha),
    queryFn: () =>
      api.get<{ file_path?: string; content?: string }[]>(`${MICROFRONTENDS}/files-at-commit`, {
        params: { repo_id: repoId, commit_sha: sha },
      }),
    enabled: Boolean(repoId && sha),
    staleTime: Infinity,
  });

  return {
    files: (query.data ?? [])
      .filter((file): file is { file_path: string; content?: string } => Boolean(file.file_path))
      .map((file) => ({ path: file.file_path, content: file.content ?? "" })),
    isLoading: query.isLoading,
    error: errorMessage(query.error, "microfrontends.filesFailed"),
  };
}

/**
 * Откат к версии: снимок выбранного коммита восстанавливается в ветку
 * `u-gen`. Живым он от этого не станет — до публикации в master
 * пользователь видит прежнее.
 */
export function useRevertMicrofrontend() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ repoId, sha, id }: { repoId: string; sha: string; id: string }) =>
      api.post<unknown>(`${MICROFRONTENDS}/revert`, {
        repo_id: repoId,
        commit_sha: sha,
        function_id: id,
      }),
    onError: (error) => reportError(error, "microfrontends.revertFailed"),
    onSuccess: async (_data, { repoId }) => {
      toast.success(i18n.t("microfrontends.reverted"));
      await queryClient.invalidateQueries({ queryKey: keys.microfrontends.commits(repoId) });
      await queryClient.invalidateQueries({
        queryKey: keys.microfrontends.promoteChanges(repoId),
      });
    },
  });
}

/**
 * Есть ли что публиковать: сравнение `u-gen` с тем, что уже уехало
 * в master.
 *
 * `everPromoted: false` — публикации не было ни разу, и это не то же
 * самое, что «есть изменения»: сравнивать не с чем. Бэкенд в этом
 * случае отдаёт `hasChanges: true` (`gitlab.go:368`), и мы повторяем
 * его решение — публиковать и правда есть что.
 */
export function usePromoteChanges(repoId: string) {
  const query = useQuery({
    queryKey: keys.microfrontends.promoteChanges(repoId),
    queryFn: () =>
      api.get<{ hasChanges?: boolean; everPromoted?: boolean }>(
        `${MICROFRONTENDS}/promote/check-changes`,
        { params: { repo_id: repoId } },
      ),
    enabled: Boolean(repoId),
  });

  return {
    hasChanges: query.data?.hasChanges === true,
    everPromoted: query.data?.everPromoted === true,
    isLoading: query.isLoading,
  };
}

/**
 * Публикация: `u-gen` уезжает в master и запускается сборка.
 *
 * Ответ — не результат, а расписка: `{status: "pending", pipeline_id}`.
 * Сборка идёт минутами, и следить за ней приходится опросом —
 * см. usePipelineStatus.
 */
export function usePromoteMicrofrontend() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (item: Microfrontend) =>
      api.post<{ status?: string; pipeline_id?: number }>(`${MICROFRONTENDS}/promote`, {
        repo_id: Number(item.repoId),
        mcp_project_id: item.mcpProjectId,
        function_id: item.id,
      }),
    onError: (error) => reportError(error, "microfrontends.promoteFailed"),
    onSuccess: async (_data, item) => {
      await queryClient.invalidateQueries({
        queryKey: keys.microfrontends.promoteChanges(item.repoId),
      });
    },
  });
}

/** Сборка идёт минутами — опрашиваем раз в пять секунд, как советует сам бэкенд. */
const PIPELINE_POLL_MS = 5000;

export function usePipelineStatus(repoId: string, pipelineId: string) {
  const query = useQuery({
    queryKey: keys.microfrontends.pipeline(repoId, pipelineId),
    queryFn: () =>
      api.get<{ status?: string }>(`${MICROFRONTENDS}/promote/pipeline-status/${pipelineId}`, {
        params: { repo_id: repoId },
      }),
    enabled: Boolean(repoId && pipelineId),
    // Опрос прекращается сам, когда сборка кончилась — успехом или нет.
    refetchInterval: (query) =>
      isPipelineDone(query.state.data?.status ?? "") ? false : PIPELINE_POLL_MS,
  });

  return { status: query.data?.status ?? "" };
}

/** Конечные состояния пайплайна GitLab. На них опрос останавливается. */
export function isPipelineDone(status: string): boolean {
  return ["success", "failed", "canceled", "skipped", "manual"].includes(status);
}
