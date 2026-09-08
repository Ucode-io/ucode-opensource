import type { Field } from "@/features/table";
import type { Item } from "./types";

/**
 * Условная видимость поля в карточке: показывать его, только когда
 * значение СОСЕДНЕГО поля той же записи совпало с заданным.
 *
 * Формат чужой и уже лежит в живых проектах — читаем его, а не
 * придумываем свой (`attributes` поля, которое прячут):
 *
 *   hide_path_field  слаг поля, за которым следим. Пусто — правила нет
 *   hide_path        ожидаемое значение: строка или массив строк
 *   type             «min» | «max» — сравнение по числу вместо равенства
 *
 * **Имя обманывает.** `hide_path` звучит как «когда прятать», а работает
 * наоборот: единственное место старой админки, которое эту настройку
 * ПРИМЕНЯЕТ (`views/Objects/NewMainInfo.jsx:44–75`), оставляет поле
 * в списке, когда условие совпало, и убирает, когда нет. Мы повторяем
 * поведение, а не название: наружу это «показывать, когда…».
 *
 * Настройка есть в обеих формах старой админки, но применяет её только
 * предыдущее поколение карточки; в нынешнем поле настраивается и
 * не прячется. То есть это не «доделать как у них», а «сделать впервые».
 *
 * Две вещи сделаны иначе, обе — исправления:
 *
 *   1. Сравнение по СТРОКОВОМУ виду, а не `===`. Ожидаемое значение
 *      человек набирает в поле ввода, то есть строкой всегда, а в записи
 *      лежит число или булево: `"5" === 5` — ложь, и правило на числовом
 *      поле не срабатывало никогда.
 *   2. Набор сравнивается как МНОЖЕСТВО, а не поэлементно. Порядок
 *      вариантов в MULTISELECT задаёт тот, кто их отмечал, и те же две
 *      метки в другом порядке — то же самое условие.
 */

/** Значение поля → набор строк. Одиночное значение — набор из одного. */
function values(value: unknown): string[] {
  const list = Array.isArray(value) ? value : [value];

  return list
    .filter((item) => item !== null && item !== undefined && item !== "")
    .map((item) => String(item));
}

function sameSet(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false;

  const set = new Set(b);
  return a.every((item) => set.has(item));
}

/**
 * Показывать ли поле при таких значениях записи.
 *
 * Записи нет вовсе (карточка ещё едет) — показываем: прятать поля
 * по отсутствующим значениям значит показать пустую карточку.
 */
export function isFieldVisible(field: Field, row: Item | undefined): boolean {
  const watched = field.attributes["hide_path_field"];
  if (typeof watched !== "string" || !watched) return true;
  if (!row) return true;

  const actual = values(row[watched]);
  const compare = field.attributes["type"];

  /*
   * Числовое сравнение. Границы ИСКЛЮЧАЮЩИЕ — так их и считает старая
   * админка: «min» показывает поле, когда значение БОЛЬШЕ заданного,
   * «max» — когда меньше.
   *
   * Ключ `type` тот же, в котором у поля FORMULA лежит вид агрегата
   * (SUMM|MAX|AVG). Разойтись они не могут: одно поле не бывает
   * одновременно агрегатом и числовым условием — форма у FORMULA
   * этого выбора не предлагает, — а чужие значения сюда не пройдут:
   * ниже проверяются ровно две строки.
   */
  if (compare === "min" || compare === "max") {
    const expected = Number(field.attributes["hide_path"]);
    const value = Number(actual[0]);
    if (!Number.isFinite(expected) || !Number.isFinite(value)) return false;

    return compare === "min" ? value > expected : value < expected;
  }

  return sameSet(actual, values(field.attributes["hide_path"]));
}

export function visibleFields(fields: Field[], row: Item | undefined): Field[] {
  return fields.filter((field) => isFieldVisible(field, row));
}
