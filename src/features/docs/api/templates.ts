import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { keys } from "@/shared/lib/query-keys";
import { reportError } from "@/shared/lib/toast";

/**
 * Шаблоны документов таблицы: печатная форма записи.
 *
 * Шаблон — это файл .docx, в котором проставлены переменные вида
 * `{slug}`. Печать подставляет в него значения строки и отдаёт PDF.
 * Синтаксис не наш и не бэкенда — его понимает служба генерации
 * документов, и старая админка подсказывает ровно его
 * (`DocumentTemplates/components/Variables/index.jsx:40`).
 *
 * Своего редактора .docx у нас нет намеренно. Старая админка встраивает
 * ONLYOFFICE (`components/Editor/index.jsx`) — это отдельный сервер
 * документов, а токен ему подписывается ПРЯМО В БРАУЗЕРЕ секретом
 * `my_jwt_secret`, лежащим в исходнике. Мы принимаем готовый файл:
 * шаблон договора делают в Word, а не в админке.
 */
export type DocTemplate = {
  id: string;
  title: string;
  tableSlug: string;
  /**
   * Адрес файла шаблона. Бэкенд хранит его БЕЗ схемы («cdn…/x.docx»),
   * и печать ждёт полный адрес — склейка одна на всё приложение, здесь.
   */
  fileUrl: string;
};

type DocTemplateDto = {
  id?: string;
  title?: string;
  table_slug?: string;
  file_url?: string;
};

type ListDto = { docx_templates?: DocTemplateDto[] | null };

/** Ссылка на файл шаблона: бэкенд отдаёт её без «https://». */
export function templateUrl(fileUrl: string): string {
  if (!fileUrl) return "";
  return /^https?:\/\//i.test(fileUrl) ? fileUrl : `https://${fileUrl}`;
}

function toTemplate(dto: DocTemplateDto): DocTemplate {
  return {
    id: dto.id ?? "",
    title: dto.title ?? "",
    tableSlug: dto.table_slug ?? "",
    fileUrl: dto.file_url ?? "",
  };
}

/** Сотни печатных форм у одной таблицы не бывает — страниц нет. */
const LIMIT = 100;

export function useDocTemplates(tableSlug: string | undefined) {
  const slug = tableSlug ?? "";

  const query = useQuery({
    queryKey: keys.docs.templates(slug),
    queryFn: () =>
      api.get<ListDto>("/v2/docx-template", {
        // Имя параметра через дефис — так его читает шлюз
        // (docx_template.go, `c.Query("table-slug")`).
        params: { "table-slug": slug, limit: LIMIT, offset: 0 },
      }),
    enabled: Boolean(slug),
    select: (dto): DocTemplate[] => (dto.docx_templates ?? []).map(toTemplate),
  });

  return { templates: query.data ?? [], isLoading: query.isLoading };
}

/**
 * Завести шаблон: файл уже загружен, сюда приезжает его адрес.
 *
 * Шлюз по этому адресу САМ скачивает файл, конвертирует его в PDF
 * через стороннюю службу и сохраняет обе ссылки (docx_template.go:97).
 * Поэтому адрес должен быть доступен снаружи — путь в хранилище
 * не подойдёт.
 */
export function useCreateDocTemplate(tableSlug: string | undefined) {
  const queryClient = useQueryClient();
  const slug = tableSlug ?? "";

  return useMutation({
    mutationFn: ({ title, fileUrl }: { title: string; fileUrl: string }) =>
      api.post<unknown>("/v2/docx-template", {
        title,
        table_slug: slug,
        file_url: fileUrl,
      }),
    onError: (error) => reportError(error, "common.createFailed"),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.docs.templates(slug) }),
  });
}

export function useDeleteDocTemplate(tableSlug: string | undefined) {
  const queryClient = useQueryClient();
  const slug = tableSlug ?? "";

  return useMutation({
    mutationFn: (template: DocTemplate) =>
      api.delete<unknown>(`/v2/docx-template/${template.id}`),
    onError: (error) => reportError(error, "common.deleteFailed"),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: keys.docs.templates(slug) }),
  });
}

/**
 * Напечатать запись: значения строки уезжают в шаблон, обратно
 * приезжает PDF.
 *
 * Ответ — не JSON, а сам файл (`Content-Type: application/pdf`,
 * docx_template.go:911), поэтому `responseType: "blob"`. Открываем его
 * новой вкладкой, а не скачиваем: печатную форму сначала смотрят.
 *
 * Строку целиком, а не guid: шлюз берёт из тела значения как есть и
 * дочитывает только связанные строки — по колонкам, кончающимся на
 * `_id` (docx_template.go:830). Что не прислали, того в документе
 * не будет.
 */
export function usePrintItem(tableSlug: string | undefined) {
  const slug = tableSlug ?? "";

  return useMutation({
    mutationFn: async ({
      template,
      row,
    }: {
      template: DocTemplate;
      row: Record<string, unknown>;
    }) => {
      const blob = await api.post<Blob>(
        "/v2/docx-template/convert/pdf",
        { table_slug: slug, data: row },
        { params: { link: templateUrl(template.fileUrl) }, responseType: "blob" },
      );

      /*
       * Адрес живёт до закрытия вкладки: отзывать его сразу нельзя —
       * вкладка не успеет прочитать, а держать ссылку в состоянии ради
       * освобождения пары мегабайт значит хранить её до конца сеанса.
       */
      window.open(URL.createObjectURL(blob), "_blank", "noopener");
    },
    onError: (error) => reportError(error, "docs.printFailed"),
  });
}
