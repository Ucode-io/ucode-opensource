import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toConditions, type Filters } from "@/features/item";
import { api } from "@/shared/api/client";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { reportError, toast } from "@/shared/lib/toast";

/**
 * Обмен таблицы с Excel.
 *
 * Выгрузка — один запрос: бэкенд складывает файл в хранилище и отдаёт
 * ссылку. Загрузка — три: файл, чтение его заголовков, запись в таблицу
 * по карте «столбец файла → поле таблицы». Эту карту нельзя угадать —
 * шапка в файле человеческая, а не слаги, — поэтому между вторым и
 * третьим шагом обязательно стоит человек.
 */

/** Ответ выгрузки: путь в хранилище, иногда уже со схемой. */
type ExcelLinkDto = { link?: string };

/** Ответ чтения файла: заголовки столбцов, в порядке файла. */
type ExcelRowsDto = { rows?: string[] };

/** Ответ загрузки файла: имя, под которым он лёг в хранилище. */
type UploadedDto = { filename?: string };

/**
 * Выгрузка в Excel.
 *
 * Выгружается то, что видно: колонки view и текущий отбор с поиском.
 * Выгрузка «всего» из экрана с фильтром выглядит как потеря фильтра,
 * а не как удобство.
 *
 * Ссылка приходит без схемы («cdn.host/file.xlsx»), поэтому она
 * достраивается здесь. Скачивание — обычная ссылка с download, а не
 * fetch с Blob: файл может быть на десятки мегабайт, и тянуть его
 * в память ради того же результата незачем.
 */
export function useExportExcel(tableSlug: string | undefined) {
  return useMutation({
    mutationFn: async ({
      fieldIds,
      filters,
      search,
    }: {
      /** id колонок view — в том же порядке, в каком они на экране. */
      fieldIds: string[];
      filters: Filters;
      search: string;
    }) => {
      const dto = await api.post<ExcelLinkDto>(`/v1/object/excel/${tableSlug ?? ""}`, {
        data: {
          ...toConditions(filters),
          field_ids: fieldIds,
          view_fields: fieldIds,
          language: i18n.language,
          ...(search.trim() ? { search: search.trim() } : {}),
        },
      });

      const link = dto.link ?? "";
      if (!link) throw new Error("Бэкенд не вернул ссылку на файл");

      return /^https?:\/\//i.test(link) ? link : `https://${link}`;
    },

    onError: (error) => reportError(error, "view.exportFailed"),
    onSuccess: (link) => {
      const anchor = document.createElement("a");
      anchor.href = link;
      anchor.download = "";
      anchor.click();
    },
  });
}

/**
 * Загрузка файла и чтение его шапки.
 *
 * Идентификатор файла — его имя без расширения: так его ждёт и
 * `/v1/excel/{id}`, и запись в таблицу. Отрезается здесь один раз,
 * а не в каждом вызывающем месте.
 */
export function useReadExcel() {
  return useMutation({
    mutationFn: async (file: File) => {
      const body = new FormData();
      body.append("file", file);

      const uploaded = await api.post<UploadedDto>("/v1/upload", body, {
        // Без этого axios сериализует FormData в JSON — см. api/files.ts.
        headers: { "Content-Type": "multipart/form-data" },
      });

      const excelId = (uploaded.filename ?? "").replace(/\.[^.]+$/, "");
      if (!excelId) throw new Error("Бэкенд не вернул имя файла");

      const dto = await api.get<ExcelRowsDto>(`/v1/excel/${excelId}`);

      return { excelId, columns: (dto.rows ?? []).filter(Boolean) };
    },

    onError: (error) => reportError(error, "view.importFailed"),
  });
}

/**
 * Запись прочитанного файла в таблицу.
 *
 * Карта идёт в сторону «столбец файла → id поля»: столбец в файле
 * ровно один раз, а одно поле заполнить из двух столбцов нельзя.
 * Неотмеченные столбцы в карту не попадают и просто игнорируются.
 */
export function useImportExcel(tableSlug: string | undefined) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ excelId, mapping }: { excelId: string; mapping: Record<string, string> }) =>
      api.post<unknown>(`/v1/excel/excel_to_db/${excelId}`, {
        data: mapping,
        table_slug: tableSlug ?? "",
      }),

    onError: (error) => reportError(error, "view.importFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("view.imported"));
      await queryClient.invalidateQueries({ queryKey: keys.items.table(tableSlug ?? "") });
    },
  });
}
