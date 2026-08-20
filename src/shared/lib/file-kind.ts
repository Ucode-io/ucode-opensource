/**
 * Что лежит по ссылке — и как его показать.
 *
 * Тип берётся из расширения, а не из ответа сервера: в ячейке хранится
 * готовый адрес, и больше о файле не известно ничего. Ручка загрузки
 * отдаёт `link`, `title` и `id` — MIME-типа в ответе нет вовсе
 * (features/item/api/files).
 */

export type FileKind = "image" | "video" | "audio" | "pdf" | "other";

const KINDS: Record<string, FileKind> = {
  jpg: "image",
  jpeg: "image",
  png: "image",
  gif: "image",
  webp: "image",
  avif: "image",
  bmp: "image",
  ico: "image",
  svg: "image",

  mp4: "video",
  webm: "video",
  ogv: "video",
  mov: "video",
  m4v: "video",

  mp3: "audio",
  wav: "audio",
  ogg: "audio",
  m4a: "audio",
  aac: "audio",

  pdf: "pdf",

  /*
   * Расширения, которых не бывает: это ТИП ПОЛЯ, дописанный при
   * загрузке вместо расширения — «..._Fitspiration(1).PHOTO».
   * Встречается в живых данных чаще, чем хотелось бы, и без этих
   * строк картинка открывалась бы карточкой «скачать файл».
   */
  photo: "image",
  multi_image: "image",
  custom_image: "image",
  video: "video",
};

export function fileKind(url: string): FileKind {
  const name = fileName(url);
  const dot = name.lastIndexOf(".");
  if (dot < 1) return "other";

  return KINDS[name.slice(dot + 1).toLowerCase()] ?? "other";
}

/** Имя файла из ссылки. Параметры запроса в имени не нужны. */
export function fileName(url: string): string {
  const path = url.split(/[?#]/)[0] ?? url;
  const last = path.split("/").filter(Boolean).pop() ?? url;

  try {
    return decodeURIComponent(last);
  } catch {
    // Битая последовательность %XX — показываем как есть.
    return last;
  }
}
