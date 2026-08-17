import { useMutation } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { reportError } from "@/shared/lib/toast";

/**
 * Загрузка файлов для ячеек PHOTO, FILE, VIDEO и их множественных
 * вариантов.
 *
 * Ручка отдаёт не адрес, а ПУТЬ в хранилище («media/1_photo.png»), и
 * читается он с другого хоста — CDN. Склейка живёт здесь одна на всё
 * приложение: в старом ucode `VITE_CDN_BASE_URL + res.link` было
 * написано в каждом из семи загрузчиков, и один из них склеивал не тот
 * ключ ответа.
 *
 * В ячейке хранится готовый адрес, а не путь: так его сохранили все
 * прежние версии, и переучивать данные ради красоты нельзя — половина
 * строк осталась бы с адресами, половина с путями.
 */
type UploadedDto = { link?: string; id?: string; title?: string };

/** Папка в хранилище. Поле может назначить свою — attributes.path. */
const DEFAULT_FOLDER = "media";

export function uploadFolder(attributes: Record<string, unknown>): string {
  const path = attributes["path"];
  return typeof path === "string" && path.trim() ? path.trim() : DEFAULT_FOLDER;
}

export function fileUrl(link: string): string {
  if (!link) return "";
  // Ответ иногда уже содержит схему — тогда это готовый адрес.
  if (/^https?:\/\//i.test(link)) return link;

  return `${import.meta.env.VITE_CDN_URL.replace(/\/+$/, "")}/${link.replace(/^\/+/, "")}`;
}

/**
 * Файлы уходят по одному — ручка принимает один `file` за запрос, —
 * но параллельно: выбрали пять картинок, ждём самую долгую, а не сумму.
 *
 * Значение ячейки при этом обновляется ОДИН раз, готовым списком:
 * писать его после каждого файла значит слать пять PUT на одну правку
 * и получить гонку, в которой побеждает не последний ответ, а самый
 * быстрый.
 */
export function useUploadFiles() {
  return useMutation({
    mutationFn: ({ files, folder }: { files: File[]; folder: string }) =>
      Promise.all(
        files.map((file) => {
          const body = new FormData();
          body.append("file", file);

          return api
            .post<UploadedDto>(`/v1/files/folder_upload`, body, {
              params: { folder_name: folder },
              /*
               * Заголовок обязателен: у клиента по умолчанию стоит
               * application/json, а axios на таком заголовке превращает
               * FormData в JSON — файл до бэкенда не доезжает. Границу
               * (boundary) браузер подставит сам.
               */
              headers: { "Content-Type": "multipart/form-data" },
            })
            .then((dto) => fileUrl(dto.link ?? ""));
        }),
      ).then((urls) => urls.filter(Boolean)),

    onError: (error) => reportError(error, "cell.uploadFailed"),
  });
}
