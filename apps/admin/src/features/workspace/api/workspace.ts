import { useMemo } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api, authApi } from "@/shared/api/client";
import { session } from "@/shared/api/session";
import { useSession } from "@/shared/api/use-session";
import { keys } from "@/shared/lib/query-keys";
import { useUi } from "@/shared/lib/ui-store";
import type { Company, DataLanguage, Environment, Project } from "../model/types";
import type { CompaniesDto, EnvironmentsDto, ProjectDetailDto, ProjectsDto } from "./dto";

/**
 * Рабочее пространство: компании → проекты → окружения.
 *
 * Списки грузятся по мере раскрытия, а не все сразу: у пользователя может
 * быть одна компания и десяток проектов, и тянуть окружения всех проектов
 * ради выпадающего списка незачем.
 */

export function useCompanies(enabled = true) {
  // Список компаний фильтруется по владельцу: без owner_id сервер
  // возвращает пустой ответ, а не все компании.
  const ownerId = useSession().getUserId();

  return useQuery({
    queryKey: keys.workspace.companies(ownerId),
    queryFn: () => api.get<CompaniesDto>("/v1/company", { params: { owner_id: ownerId } }),
    enabled: enabled && Boolean(ownerId),
    staleTime: 5 * 60_000,
    select: (data): Company[] =>
      (data.companies ?? [])
        .filter((dto) => dto.id)
        .map((dto) => ({ id: dto.id!, name: dto.title || dto.name || "—" })),
  });
}

export function useProjects(companyId: string, enabled = true) {
  return useQuery({
    queryKey: keys.workspace.projects(companyId),
    queryFn: () => api.get<ProjectsDto>("/v1/company-project", { params: { company_id: companyId } }),
    enabled: enabled && Boolean(companyId),
    staleTime: 5 * 60_000,
    select: (data): Project[] =>
      (data.projects ?? [])
        .map((dto) => ({ id: dto.id || dto.project_id || "", name: dto.title || dto.name || "—" }))
        .filter((project) => project.id),
  });
}

/**
 * Языки данных текущего проекта. На них хранятся значения мультиязычных
 * полей и подписи вариантов — см. DataLanguage.
 *
 * Первый язык считается основным: так же его выбирает бэкенд, заводя
 * поле. Выбранный человеком язык живёт в ui-store — переключатель стоит
 * и над таблицей, и в карточке, и выбор обязан пережить переход между
 * ними. Язык из хранилища проверяется по набору проекта: язык, убранный
 * из настроек, иначе показывал бы пустые подписи навсегда.
 */
/**
 * Карточка проекта — ОДИН запрос на всех, кто её читает.
 *
 * Её данные нужны в трёх местах сразу: языки данных (везде), логотип
 * в шапке сайдбара и настройки проекта. Ключ поэтому один, а каждый
 * потребитель берёт из ответа своё — иначе тот же адрес грузился бы
 * по разу на потребителя, как это было в старом ucode с четырьмя
 * именами кэша для одних и тех же строк.
 */
export function useProjectDetail() {
  const projectId = useSession().getProjectId() ?? "";

  return useQuery({
    queryKey: keys.workspace.project(projectId),
    queryFn: () => api.get<ProjectDetailDto>(`/v1/company-project/${projectId}`),
    enabled: Boolean(projectId),
    // Карточку проекта меняют в настройках, а не по ходу работы.
    staleTime: 5 * 60_000,
  });
}

export function useDataLanguages() {
  const chosen = useUi((state) => state.dataLanguage);
  const setDataLanguage = useUi((state) => state.setDataLanguage);

  // Своего запроса нет: языки — это часть карточки проекта, которую
  // и так грузит useProjectDetail.
  const { data } = useProjectDetail();
  const languages = useMemo(() => (data ? toLanguages(data) : NO_LANGUAGES), [data]);

  const known = languages.some((language) => language.code === chosen);

  return {
    languages,
    current: (known ? chosen : languages[0]?.code) ?? "",
    setCurrent: setDataLanguage,
  };
}

const NO_LANGUAGES: DataLanguage[] = [];

function toLanguages(data: ProjectDetailDto): DataLanguage[] {
  return (data.language ?? [])
    .filter((dto) => dto.short_name)
    .map((dto) => ({
      code: dto.short_name!,
      name: dto.name || dto.short_name!,
      nativeName: dto.native_name || dto.name || dto.short_name!,
    }));
}

export function useEnvironments(projectId: string, enabled = true) {
  return useQuery({
    queryKey: keys.workspace.environments(projectId),
    queryFn: () =>
      api.get<EnvironmentsDto>("/v1/environment", { params: { project_id: projectId } }),
    enabled: enabled && Boolean(projectId),
    staleTime: 5 * 60_000,
    select: (data): Environment[] =>
      (data.environments ?? [])
        .filter((dto) => dto.id)
        .map((dto) => ({
          id: dto.id!,
          name: dto.name || "—",
          projectId: dto.project_id ?? projectId,
          color: dto.display_color ?? "",
          description: dto.description?.trim() ?? "",
          isDefault: dto.default ?? false,
        })),
  });
}

/**
 * Переключение окружения. Это переход, а не смена фильтра: бэкенд
 * мутирует ту же серверную сессию и выдаёт новую пару токенов
 * (V2RefreshTokenForEnv), см. docs/adr/0001.
 *
 * Отличия от старой версии — это не стиль, а исправленные ошибки:
 *
 *   1. Состояние меняется ПОСЛЕ успеха. Старый код диспатчил новый
 *      environmentId до запроса: при отказе интерфейс показывал одно
 *      окружение, а токены оставались от другого.
 *   2. Кэш чистится, а не перезагружается страница. Старый код звал
 *      window.location.reload() — весь бандл заново ради сброса данных.
 *   3. Ошибка возвращается вызывающему коду. Старый код глушил её
 *      в console.log, и пользователь не узнавал, что переключение не прошло.
 */
type SwitchTarget = { projectId: string; environmentId: string };

export function useSwitchEnvironment() {
  return useMutation({
    mutationFn: async ({ projectId, environmentId }: SwitchTarget) => {
      const refresh = session.getRefresh();
      if (!refresh) throw new Error("Нет refresh-токена");

      const data = await authApi.put<{
        token?: { access_token: string; refresh_token: string };
        environment_id?: string;
      }>(
        "/v2/refresh",
        { refresh_token: refresh, env_id: environmentId, project_id: projectId },
        { params: { for_env: true } },
      );

      if (!data.token) throw new Error("Бэкенд не вернул токен");

      // Проект и окружение записываются вместе: иначе между двумя
      // записями случится рендер с новым окружением и старым проектом.
      session.set({
        access: data.token.access_token,
        refresh: data.token.refresh_token,
        environmentId: data.environment_id ?? environmentId,
        projectId,
      });
    },

    /*
     * Кэш НЕ чистим. Окружение входит в ключи запросов, поэтому данные
     * разных окружений и так лежат отдельно — а queryClient.clear()
     * здесь ломал сайдбар: он сносил запрос у уже смонтированного
     * подписчика, и тот оставался в загрузке, не отправив новый.
     *
     * Старые ключи просто устареют и уйдут по gcTime.
     */
  });
}

/** Создание компании. Валидация имени — на форме, здесь только запрос. */
export function useCreateCompany() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (name: string) => api.post<unknown>("/v1/company", { name }),
    // Инвалидируем весь раздел: ключ списка зависит от владельца.
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.workspace.all }),
  });
}
