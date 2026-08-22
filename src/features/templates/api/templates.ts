import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { reportError, toast } from "@/shared/lib/toast";

/**
 * Шаблоны проекта: готовый набор таблиц с полями, связями, view,
 * раскладками и действиями.
 *
 * Ручка `POST /v1/template/execute` разворачивает шаблон в проект.
 * Тело она читает своё (`ExecuteTemplate`: `id`, `tables`, `menus`),
 * а остальное молча выбрасывает — старая админка слала туда ещё имя,
 * описание и `menu_id`, и ни одно из них до обработчика не доезжает
 * (`api/handlers/v1/template.go:29`).
 *
 * Содержимое шаблона мы не разбираем и не собираем: что пришло списком
 * таблиц, то и уходит обратно. Разбирать его — значит держать у себя
 * копию модели всего конструктора: поля, связи, view, раскладки,
 * действия и строки. Она устареет на первой же правке бэкенда.
 */

type TemplateDto = {
  id?: string;
  name?: string;
  description?: string;
  photo?: string;
  /** Содержимое шаблона. Не разбираем — см. выше. */
  tables?: unknown;
};

type TemplatesResponse = { templates?: TemplateDto[] } | TemplateDto[];

export type Template = {
  id: string;
  name: string;
  description: string;
  photo: string;
  tables: unknown;
};

export function useTemplates(enabled: boolean) {
  const query = useQuery({
    queryKey: keys.templates.list(),
    queryFn: () => api.get<TemplatesResponse>("/v1/template"),
    enabled,
    staleTime: 5 * 60_000,
  });

  return {
    templates: toTemplates(query.data),
    isLoading: query.isLoading,
    error: query.error,
  };
}

/**
 * Развернуть шаблон в проект.
 *
 * После успеха перезапрашиваем меню целиком: шаблон заводит таблицы
 * и пункты пачкой, и какие именно — знает только сервер.
 */
export function useApplyTemplate() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (template: Template) =>
      api.post("/v1/template/execute", { id: template.id, tables: template.tables }),
    onError: (error) => reportError(error, "templates.failed"),
    onSuccess: async () => {
      toast.success(i18n.t("templates.applied"));
      await queryClient.invalidateQueries({ queryKey: keys.menus.all });
    },
  });
}

export type TemplateDraft = {
  name: string;
  description: string;
  /** Папка, из которой делают шаблон: её дерево уезжает в шаблон целиком. */
  menuId: string;
  /** Идентификаторы таблиц, попадающих в шаблон. */
  tables: string[];
  /** Взять ещё и строки — сразу у всех таблиц или ни у одной, см. ниже. */
  withRows: boolean;
};

/**
 * Сделать шаблон из папки.
 *
 * Содержимое таблиц собирает САМ ШЛЮЗ: по каждому `id` из тела он
 * дочитывает поля, связи, view, раскладки и действия, а по `menu_id` —
 * дерево пунктов (`api/handlers/v1/template.go:210`). От нас нужны
 * только имя, папка и отмеченные таблицы — той копии модели конструктора,
 * которой мы избегаем, здесь не требуется.
 *
 * `with_rows` — забрать ещё и строки. Шлюз берёт первую сотню
 * (`limit: 100` там же, строка 110): шаблон — это заготовка, а не выгрузка.
 *
 * Флаг ОДИН НА ШАБЛОН, а не по таблице, хотя тело принимает его у каждой.
 * Прочитанные строки шлюз держит в переменной, объявленной ВНЕ цикла
 * по таблицам (`template.go:106`), а в шаблон кладёт безусловно
 * (строка 345): таблица без `with_rows`, идущая следом за таблицей с ним,
 * получает ЧУЖИЕ строки. Смешанного выбора мы не отправляем — тогда
 * либо у каждой свои строки, либо ни у кого никаких.
 * См. [backend-notes](../../../../docs/backend-notes.md), «Шаблоны».
 */
export function useCreateTemplate() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (draft: TemplateDraft) =>
      api.post("/v1/template", {
        name: draft.name,
        description: draft.description,
        menu_id: draft.menuId,
        tables: draft.tables.map((id) => ({ id, with_rows: draft.withRows })),
      }),
    onError: (error) => reportError(error, "templates.createFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("templates.created"));
      await queryClient.invalidateQueries({ queryKey: keys.templates.all });
    },
  });
}

/** Ответ приходит и списком, и объектом со списком — смотря по ветке шлюза. */
export function toTemplates(data: TemplatesResponse | undefined): Template[] {
  const list = Array.isArray(data) ? data : (data?.templates ?? []);

  return list
    .filter((dto): dto is TemplateDto & { id: string } => Boolean(dto.id))
    .map((dto) => ({
      id: dto.id,
      name: dto.name?.trim() || dto.id,
      description: dto.description?.trim() ?? "",
      photo: dto.photo?.trim() ?? "",
      tables: dto.tables ?? [],
    }));
}
