/**
 * Показ чисел — одно место на всё приложение, как у дат
 * (см. date-value).
 *
 * Разделители разрядов нужны длинным суммам: «1 234 567» читается
 * с одного взгляда, «1234567» приходится считать глазами. Старая
 * админка ставила их только в карточках доски и timeline
 * (`CellElementGenerator.jsx:114`), а в таблице показывала число как
 * есть — одно и то же значение выглядело на двух экранах по-разному.
 * Поэтому здесь общий форматтер, и зовут его все читатели числа:
 * ячейка, формула, подпись связанной строки.
 *
 * Разделитель берётся из языка ИНТЕРФЕЙСА: в русском это пробел,
 * в английском запятая. Это подпись, а не значение — в колонке
 * по-прежнему лежит число, и в поле ввода оно правится без пробелов.
 */

/** Форматтер стоит дорого, а ячеек в таблице сотни. */
const formatters = new Map<string, Intl.NumberFormat>();

function formatter(locale: string): Intl.NumberFormat {
  let cached = formatters.get(locale);

  if (!cached) {
    /*
     * Дробную часть не режем. `maximumFractionDigits` у Intl по
     * умолчанию три, то есть 1.23456 молча превратилось бы в «1,235»:
     * показ округлил бы то, что лежит в колонке. 20 — потолок Intl
     * и заведомо больше, чем помещается в double.
     */
    cached = new Intl.NumberFormat(locale, { maximumFractionDigits: 20 });
    formatters.set(locale, cached);
  }

  return cached;
}

/**
 * Число с разделителями разрядов. Не число — как есть: в колонке
 * VARCHAR (FORMULA_FRONTEND, RANDOM_NUMBERS) лежит что угодно, и
 * подменять непонятное значение на «NaN» нельзя.
 */
export function formatNumber(value: unknown, locale: string): string {
  if (typeof value === "number") {
    return Number.isFinite(value) ? formatter(locale).format(value) : String(value);
  }

  if (typeof value !== "string") return String(value ?? "");

  const text = value.trim();
  const number = Number(text);

  // Пустая строка у Number — это ноль, а не «не число»: отсеиваем сами,
  // иначе пустое значение показалось бы нулём.
  if (!text || !Number.isFinite(number)) return value;

  return formatter(locale).format(number);
}
