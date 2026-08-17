import type { Field } from "@/features/table";
import { editorKind } from "./cell-kind";
import { isBlank } from "./cell-value";
import type { Item } from "./types";

/**
 * Проверка значения ячейки перед отправкой.
 *
 * Две настройки поля, и обе живут в схеме:
 *   required                       колонка таблицы field
 *   attributes.validation          регулярное выражение
 *   attributes.validation_message  что показать, когда не совпало
 *
 * Проверять на клиенте нужно не вместо бэкенда, а раньше него: колонка
 * с NOT NULL ответит 500 с текстом драйвера, а регулярное выражение
 * бэкенд не проверяет вовсе — оно существует только ради этой проверки.
 *
 * Текста здесь нет: `kind` переводит тот, кто показывает (см. i18n —
 * модели про язык интерфейса не знают). Сообщение админа приходит
 * готовым и переводу не подлежит — его писали на языке проекта.
 */
export type CellError = {
  kind: "required" | "pattern";
  /** Текст из настроек поля. Пусто — сказать нечего, кроме общего. */
  message: string;
};

export function cellError(field: Field, value: unknown): CellError | null {
  if (isBlank(value)) return field.required ? { kind: "required", message: "" } : null;
  if (!field.validation) return null;

  /*
   * Регулярное выражение имеет смысл только для того, что человек
   * набирает текстом. Список файлов, точку на карте или объект гнать
   * через String() значит проверять «[object Object]» — совпадёт или
   * нет, зависит от выражения, и обе ветки одинаково бессмысленны.
   */
  if (typeof value !== "string" && typeof value !== "number") return null;

  // Совпадение частичное, без якорей, — как в старой админке (rules.pattern
  // у react-hook-form). Выражения в живых проектах написаны под неё.
  return field.validation.pattern.test(String(value))
    ? null
    : { kind: "pattern", message: field.validation.message };
}

/**
 * Проверка целой строки — новой, которую ещё не отправляли.
 *
 * Правка ячейки проверяет одно поле, потому что и уезжает одно поле.
 * Новая строка уезжает целиком, и проверять её по одной ошибке за раз
 * значит заставлять человека заполнять форму столько раз, сколько в ней
 * незаполненных обязательных полей.
 *
 * Только то, что правится: у вычисляемого поля (INCREMENT_ID, FORMULA)
 * значения в черновике нет и взяться ему неоткуда — обязательность
 * такой колонки закрывает бэкенд при вставке, а не человек.
 */
export function rowErrors(columns: Field[], row: Item): Map<string, CellError> {
  const errors = new Map<string, CellError>();

  for (const field of columns) {
    if (!editorKind(field)) continue;

    const error = cellError(field, row[field.slug]);
    if (error) errors.set(field.slug, error);
  }

  return errors;
}
