import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { reportError, toast } from "@/shared/lib/toast";

/**
 * Настройки проекта: имя, языки данных, часовой пояс.
 *
 * Языки здесь — те самые [[Data Language]], на которых размечены подписи
 * полей, имена view и мультиязычные колонки. Меняя их здесь, человек
 * меняет разметку данных всего проекта, а не свой интерфейс.
 */

type LanguageDto = { id?: string; name?: string; short_name?: string; native_name?: string };
type NamedDto = { id?: string; name?: string };

type ProjectDto = {
  project_id?: string;
  company_id?: string;
  title?: string;
  logo?: string;
  language?: LanguageDto[];
  timezone?: NamedDto | null;
  currency?: NamedDto | null;
};

export type ProjectOption = { id: string; name: string };

export type ProjectSettings = {
  id: string;
  title: string;
  logo: string;
  /** Языки данных проекта — id из справочника LANGUAGE. */
  languageIds: string[];
  timezoneId: string;
  raw: Record<string, unknown>;
};

export function useProject(enabled = true) {
  const projectId = useSession().getProjectId() ?? "";

  const query = useQuery({
    queryKey: keys.settings.project(projectId),
    queryFn: () => api.get<ProjectDto>(`/v1/company-project/${projectId}`),
    enabled: enabled && Boolean(projectId),
    staleTime: 5 * 60_000,
    select: toProjectSettings,
  });

  return { project: query.data, isLoading: query.isLoading };
}

/**
 * Справочник настроек проекта: языки, часовые пояса, валюты.
 *
 * Ручка одна на три списка и различает их параметром `type`, поэтому
 * и ключ кэша включает тип: иначе языки и пояса делили бы одну ячейку.
 */
export function useProjectOptions(type: "LANGUAGE" | "TIMEZONE", enabled = true) {
  const projectId = useSession().getProjectId() ?? "";

  const query = useQuery({
    queryKey: keys.settings.options(projectId, type),
    queryFn: () =>
      api.get<SettingsResponse>("/v1/project/setting", {
        params: { "project-id": projectId, type, limit: 200 },
      }),
    enabled: enabled && Boolean(projectId),
    // Справочник меняется раз в никогда.
    staleTime: 30 * 60_000,
    select: (data): ProjectOption[] =>
      listOf(data, type)
        .filter((item) => item.id)
        .map((item) => ({ id: item.id!, name: item.name || item.id! })),
  });

  return { options: query.data ?? [], isLoading: query.isLoading };
}

/**
 * Справочник завёрнут ДВАЖДЫ: общий конверт ответа снимает http-клиент,
 * а внутри лежит ещё один `data` — `{data: {count, language: []}}`.
 * Читаем оба вида: у соседних ручек второго слоя нет.
 */
type SettingsResponse = Record<string, unknown> & { data?: Record<string, unknown> };

function listOf(body: SettingsResponse, type: string): LanguageDto[] {
  const key = type.toLowerCase();
  const inner = body.data?.[key] ?? body[key];

  return Array.isArray(inner) ? (inner as LanguageDto[]) : [];
}

export type ProjectDraft = { title?: string; languageIds?: string[]; timezoneId?: string };

/**
 * Правка проекта.
 *
 * Тело — поверх ответа: PUT принимает проект целиком, и собранное
 * заново тело стёрло бы тариф, ресурсы и баланс.
 *
 * Языки уезжают объектами, а не идентификаторами: колонка проекта
 * хранит их развёрнутыми (company_service, Project.Language), и список
 * из голых id она не примет. Берём их из справочника — там они полные,
 * вместе с short_name, по которому размечены данные.
 */
export function useUpdateProject(languages: LanguageOption[]) {
  const queryClient = useQueryClient();
  const projectId = useSession().getProjectId() ?? "";

  return useMutation({
    mutationFn: ({ project, draft }: { project: ProjectSettings; draft: ProjectDraft }) => {
      const ids = draft.languageIds ?? project.languageIds;

      return api.put<unknown>(`/v1/company-project/${projectId}`, {
        ...project.raw,
        project_id: projectId,
        ...(draft.title === undefined ? {} : { title: draft.title.trim() }),
        ...(draft.languageIds === undefined
          ? {}
          : { language: ids.map((id) => languages.find((item) => item.id === id)).filter(Boolean) }),
        ...(draft.timezoneId === undefined ? {} : { timezone: { id: draft.timezoneId } }),
      });
    },

    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("settings.saved"));
      await queryClient.invalidateQueries({ queryKey: keys.settings.project(projectId) });
      // Языки данных читает половина экранов — их список тоже устарел.
      await queryClient.invalidateQueries({ queryKey: keys.workspace.all });
    },
  });
}

/** Язык справочника целиком: id, имя и код, которым размечены данные. */
export type LanguageOption = { id: string; name: string; short_name: string; native_name: string };

/**
 * Языки справочника — с кодами. Отдельно от useProjectOptions: там
 * нужны только id и имя, а при записи проекта уезжает объект целиком.
 */
export function useLanguageOptions(enabled = true) {
  const projectId = useSession().getProjectId() ?? "";

  const query = useQuery({
    queryKey: [...keys.settings.options(projectId, "LANGUAGE"), "full"],
    queryFn: () =>
      api.get<SettingsResponse>("/v1/project/setting", {
        params: { "project-id": projectId, type: "LANGUAGE", limit: 200 },
      }),
    enabled: enabled && Boolean(projectId),
    staleTime: 30 * 60_000,
    select: (data): LanguageOption[] =>
      listOf(data, "LANGUAGE")
        .filter((item) => item.id)
        .map((item) => ({
          id: item.id!,
          name: item.name || item.id!,
          short_name: item.short_name ?? "",
          native_name: item.native_name ?? item.name ?? "",
        })),
  });

  return { languages: query.data ?? [], isLoading: query.isLoading };
}

export function toProjectSettings(dto: ProjectDto): ProjectSettings {
  return {
    id: dto.project_id ?? "",
    title: dto.title?.trim() ?? "",
    logo: dto.logo ?? "",
    languageIds: (dto.language ?? []).map((item) => item.id ?? "").filter(Boolean),
    timezoneId: dto.timezone?.id ?? "",
    raw: { ...dto },
  };
}
