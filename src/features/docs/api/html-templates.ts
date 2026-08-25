import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { reportError, toast } from "@/shared/lib/toast";
import { printableRow } from "../model/print-data";

/**
 * HTML-шаблоны таблицы — второе поколение печатной формы, живущее
 * параллельно docx.
 *
 * Отличие от docx одно и важное: шаблон здесь не файл, а РАЗМЕТКА
 * в самой записи (`html` в `html_template.proto:20`). Поэтому его
 * не загружают, а правят — и правят исходником.
 *
 * Своего редактора с кнопками мы не делаем по той же причине, по
 * которой не встраиваем ONLYOFFICE для docx (см. api/templates):
 * старая админка держит для этого CKEditor целым экраном
 * (`views/Objects/DocView`), а поле здесь — обычная строка с теми же
 * переменными `{slug}`. Кто пишет шаблон договора, тот пишет и HTML.
 *
 * **Работает только у mongo-проектов, и это не наша граница.** CRUD
 * уходит в `GetBuilderServiceByType` БЕЗ разбора типа ресурса
 * (`html_template.go:73`) — то есть всегда в mongo-сборщик; а обе
 * ручки превращения размечены `// Does Not Implemented` для PostgreSQL
 * (`view.go:97` и `:187`) и отвечают на нём `201` с пустым телом.
 * Отсюда наше правило: пустая ссылка в ответе — это отказ, и он
 * произносится вслух, а не выглядит успехом (см. useHtmlToPdf).
 */
export type HtmlTemplate = {
  id: string;
  title: string;
  tableSlug: string;
  /** Разметка шаблона с переменными `{slug}`. */
  html: string;
};

type HtmlTemplateDto = {
  id?: string;
  title?: string;
  table_slug?: string;
  html?: string;
};

/** Ключ ответа именно такой — `htmlTemplates`, camelCase посреди змеиных. */
type ListDto = { htmlTemplates?: HtmlTemplateDto[] | null };

function toTemplate(dto: HtmlTemplateDto): HtmlTemplate {
  return {
    id: dto.id ?? "",
    title: dto.title ?? "",
    tableSlug: dto.table_slug ?? "",
    html: dto.html ?? "",
  };
}

const TEMPLATES = "/v1/html-template";

export function useHtmlTemplates(tableSlug: string | undefined) {
  const slug = tableSlug ?? "";

  const query = useQuery({
    queryKey: keys.docs.htmlTemplates(slug),
    // Имя параметра со змеиным подчёркиванием — в отличие от docx,
    // где тот же отбор называется «table-slug» через дефис.
    queryFn: () => api.get<ListDto>(TEMPLATES, { params: { table_slug: slug } }),
    enabled: Boolean(slug),
    select: (dto): HtmlTemplate[] => (dto.htmlTemplates ?? []).map(toTemplate),
  });

  return { templates: query.data ?? [], isLoading: query.isLoading };
}

/**
 * Завести шаблон. Ответ — сама заведённая запись
 * (`html_template.go:23`, `data=obs.HtmlTemplate`), поэтому вызывающий
 * может открыть её на правку сразу: разметку всё равно писать, а шага
 * «найдите в списке и нажмите карандаш» между этим нет.
 */
export function useCreateHtmlTemplate(tableSlug: string | undefined) {
  const queryClient = useQueryClient();
  const slug = tableSlug ?? "";

  return useMutation({
    mutationFn: async (title: string) =>
      toTemplate(await api.post<HtmlTemplateDto>(TEMPLATES, { title, table_slug: slug, html: "" })),
    onError: (error) => reportError(error, "common.createFailed"),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.docs.htmlTemplates(slug) }),
  });
}

/**
 * Правка шаблона: имя и разметка уезжают вместе.
 *
 * Запись перезаписывается целиком — `Update` принимает `HtmlTemplate`,
 * а не набор изменённых полей, — поэтому отправляем оба поля, даже
 * когда правили одно. Своего `table_slug` тоже: без него шаблон
 * потерял бы таблицу и пропал из списка.
 */
export function useUpdateHtmlTemplate(tableSlug: string | undefined) {
  const queryClient = useQueryClient();
  const slug = tableSlug ?? "";

  return useMutation({
    mutationFn: (template: HtmlTemplate) =>
      api.put<unknown>(TEMPLATES, {
        id: template.id,
        title: template.title,
        table_slug: slug,
        html: template.html,
      }),
    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.docs.htmlTemplates(slug) }),
  });
}

export function useDeleteHtmlTemplate(tableSlug: string | undefined) {
  const queryClient = useQueryClient();
  const slug = tableSlug ?? "";

  return useMutation({
    mutationFn: (template: HtmlTemplate) => api.delete<unknown>(`${TEMPLATES}/${template.id}`),
    onError: (error) => reportError(error, "common.deleteFailed"),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.docs.htmlTemplates(slug) }),
  });
}

/**
 * Напечатать запись по HTML-шаблону.
 *
 * Ответ — не файл, а ССЫЛКА на него (`PdfBody.link`, `view.proto:156`):
 * в отличие от docx, где приезжает сам PDF. Открываем её вкладкой.
 *
 * Пустая ссылка при коде 200 — это и есть отказ postgres-проекта:
 * обработчик доходит до `HandleResponse` с неинициализированным ответом
 * (`view.go:97–101`). Молчать о нём нельзя — «напечатал и ничего
 * не произошло» читается как поломка браузера.
 */
export function useHtmlToPdf() {
  return useMutation({
    mutationFn: async ({
      template,
      row,
      lookups,
    }: {
      template: HtmlTemplate;
      row: Record<string, unknown>;
      /** Слаги полей LOOKUP: только они уезжают из ключей с «_id». */
      lookups: ReadonlySet<string>;
    }) => {
      const { link } = await api.post<{ link?: string }>("/v1/html-to-pdf", {
        html: template.html,
        data: printableRow(row, lookups),
      });

      /*
       * Отказ, а не ошибка: сеть отработала, ответ пришёл, просто
       * конвертера для postgres-проекта нет. Говорим об этом здесь,
       * а не исключением: брошенное отсюда `Error` в onError неотличимо
       * от сетевого сбоя, и «Request failed with status code 500»
       * уехало бы человеку той же строкой.
       */
      if (!link) {
        toast.error(i18n.t("docs.htmlNotSupported"));
        return;
      }

      window.open(link, "_blank", "noopener,noreferrer");
    },
    onError: (error) => reportError(error, "docs.printFailed"),
  });
}
