/** Сырые ответы админ-API. Наружу не выходят. */
export type CompanyDto = { id?: string; name?: string; title?: string; logo?: string };
export type CompaniesDto = { companies?: CompanyDto[]; count?: number };

export type ProjectDto = { id?: string; project_id?: string; title?: string; name?: string };
export type ProjectsDto = { projects?: ProjectDto[]; count?: number };

/** GET /v1/company-project/{id} — карточка проекта целиком. */
export type LanguageDto = { id?: string; name?: string; short_name?: string; native_name?: string };
export type ProjectDetailDto = { project_id?: string; title?: string; language?: LanguageDto[] };

export type EnvironmentDto = {
  id?: string;
  name?: string;
  project_id?: string;
  display_color?: string;
  default?: boolean;
};
export type EnvironmentsDto = { environments?: EnvironmentDto[]; count?: number };
