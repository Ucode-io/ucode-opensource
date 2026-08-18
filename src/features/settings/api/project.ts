import { useMemo } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useProjectDetail } from "@/features/workspace";
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
  /**
   * Наборы значков, из которых выбирают иконку пункта меню. Значения —
   * `<префикс iconify>#<имя набора>`: так их пишет старая админка, и так
   * же их читает выбор иконки.
   */
  icon_categories?: string[];
};

export type ProjectOption = { id: string; name: string };

export type ProjectSettings = {
  id: string;
  title: string;
  logo: string;
  /** Языки данных проекта — id из справочника LANGUAGE. */
  languageIds: string[];
  timezoneId: string;
  /** Валюта проекта — id из справочника CURRENCY. */
  currencyId: string;
  /** Наборы значков: `<префикс>#<имя>`, см. ProjectDto.icon_categories. */
  iconCategories: string[];
  raw: Record<string, unknown>;
};

/**
 * Настройки проекта.
 *
 * Своего запроса нет: карточку проекта грузит features/workspace, и её
 * же читают языки данных и логотип в шапке. Раньше тот же адрес лежал
 * здесь под своим ключом — то есть на каждом экране, где открыты
 * настройки, он приезжал дважды.
 */
export function useProject() {
  const query = useProjectDetail();
  const project = useMemo(
    () => (query.data ? toProjectSettings(query.data as ProjectDto) : undefined),
    [query.data],
  );

  return { project, isLoading: query.isLoading };
}

/**
 * Справочник настроек проекта: языки, часовые пояса, валюты.
 *
 * Ручка одна на три списка и различает их параметром `type`, поэтому
 * и ключ кэша включает тип: иначе языки и пояса делили бы одну ячейку.
 */
export function useProjectOptions(type: "LANGUAGE" | "TIMEZONE" | "CURRENCY", enabled = true) {
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

export type ProjectDraft = {
  title?: string;
  languageIds?: string[];
  timezoneId?: string;
  currencyId?: string;
  iconCategories?: string[];
  /** Адрес логотипа в нашем CDN. Пусто — логотипа нет. */
  logo?: string;
};

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
        /*
         * Валюта и часовой пояс уезжают объектом с одним id: остальное
         * бэкенд подставляет сам из справочника, а список из голого
         * идентификатора он не примет — как и языки.
         */
        ...(draft.currencyId === undefined ? {} : { currency: { id: draft.currencyId } }),
        ...(draft.iconCategories === undefined ? {} : { icon_categories: draft.iconCategories }),
        ...(draft.logo === undefined ? {} : { logo: draft.logo }),
      });
    },

    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("settings.saved"));
      /*
       * Один ключ на всех: карточку проекта читают и языки данных,
       * и логотип в шапке, и этот экран. Второго места, которое надо
       * было бы протухать отдельно, больше нет.
       */
      await queryClient.invalidateQueries({ queryKey: keys.workspace.project(projectId) });
    },
  });
}

/** Язык справочника целиком: id, имя и код, которым размечены данные. */
export type LanguageOption = { id: string; name: string; short_name: string; native_name: string };

/**
 * Языки справочника — с кодами.
 *
 * Отдельный разбор, но ТОТ ЖЕ ключ, что у useProjectOptions("LANGUAGE"):
 * запрос один, а каждый потребитель берёт из ответа своё. С отдельным
 * ключом тот же адрес приезжал бы дважды на одном экране.
 */
export function useLanguageOptions(enabled = true) {
  const projectId = useSession().getProjectId() ?? "";

  const query = useQuery({
    queryKey: keys.settings.options(projectId, "LANGUAGE"),
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
    currencyId: dto.currency?.id ?? "",
    iconCategories: (dto.icon_categories ?? []).filter(Boolean),
    raw: { ...dto },
  };
}

/**
 * Наборы значков iconify — справочник для настройки проекта.
 *
 * Запрос идёт МИМО нашего http-клиента: это чужой хост, ему не нужны
 * ни наш токен, ни окружение, ни разворачивание конверта. Отдельный
 * инстанс axios ради одного GET — лишняя сущность, поэтому fetch.
 *
 * Так же берёт их и старая админка (ProjectSettings.jsx): своего
 * справочника наборов у ucode нет.
 */
export type IconCollection = { value: string; label: string };

const ICONIFY_COLLECTIONS = "https://api.iconify.design/collections";

export function useIconCollections(enabled = true) {
  const query = useQuery({
    queryKey: keys.settings.iconCollections(),
    queryFn: async (): Promise<IconCollection[]> => {
      const response = await fetch(ICONIFY_COLLECTIONS);
      if (!response.ok) throw new Error(`iconify: ${response.status}`);

      const body = (await response.json()) as Record<string, { name?: string }>;

      return Object.entries(body).map(([prefix, item]) => ({
        // Формат значения — `<префикс>#<имя>`: его пишет и читает
        // старая админка, и по префиксу же запрашиваются сами значки.
        value: `${prefix}#${item?.name ?? prefix}`,
        label: item?.name ?? prefix,
      }));
    },
    enabled,
    // Наборы iconify меняются несколько раз в год.
    staleTime: 24 * 60 * 60_000,
  });

  return { collections: query.data ?? NO_COLLECTIONS, isLoading: query.isLoading };
}

const NO_COLLECTIONS: IconCollection[] = [];
