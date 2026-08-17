import { Suspense, use } from "react";
// Только тип: при сборке импорт стирается, и библиотека остаётся
// в своём куске, который грузится по требованию (см. parser).
import type { Parser } from "hot-formula-parser";
import type { Item } from "../model/types";

/**
 * FORMULA_FRONTEND: значение считает браузер на каждый показ.
 *
 * В колонке ничего не лежит — бэкенд такие поля не считает, они и
 * называются frontend. Формула написана по слагам полей ТОЙ ЖЕ строки
 * («price * count»), синтаксис excel-подобный.
 *
 * Разбирает её hot-formula-parser — тот же, что в старой админке.
 * Свой разборщик выражений был бы меньше и без зависимости, но формулы
 * в живых проектах уже написаны под эту библиотеку, вместе с её SUM,
 * IF и ROUND: сменить разборщик значит сменить смысл написанного.
 */

/*
 * Библиотека грузится один раз на всё приложение и только когда такая
 * колонка встретилась: с @handsontable/formulajs внутри это заметный
 * кусок, а поля-формулы есть далеко не у всех.
 *
 * Промис кэшируется в модуле, поэтому `use` в ячейке приостанавливает
 * рендер только на первой — дальше он уже разрешён и возвращает
 * разборщик сразу.
 */
let pending: Promise<Parser> | null = null;

function parser() {
  pending ??= import("hot-formula-parser").then((module) => new module.Parser());
  return pending;
}

export function FormulaCell({
  formula,
  row,
  locale,
  line,
}: {
  formula: string;
  row: Item;
  locale: string;
  /** truncate или перенос — как у остальных ячеек. */
  line: string;
}) {
  return (
    /* Пока библиотека едет — пусто, а не «загрузка»: строка таблицы
       не должна дёргаться из-за куска, который приезжает один раз. */
    <Suspense fallback={<span />}>
      <Value formula={formula} row={row} locale={locale} line={line} />
    </Suspense>
  );
}

function Value({
  formula,
  row,
  locale,
  line,
}: {
  formula: string;
  row: Item;
  locale: string;
  line: string;
}) {
  const engine = use(parser());

  /*
   * Значения строки — переменными, а не подстановкой в текст формулы.
   * Старая админка склеивала строку заменой слага на значение, и
   * поле с пустым значением превращало формулу в «* 2» — разбор падал
   * целиком. Переменные ещё и не путают `price` с `price_usd`:
   * подстановка требовала сортировать слаги по длине.
   */
  for (const [key, value] of Object.entries(row)) {
    engine.setVariable(key, toOperand(value));
  }

  const { error, result } = engine.parse(formula);

  // Код ошибки показываем как есть: «#NAME?» значит «в формуле слаг,
  // которого в строке нет», и это единственная подсказка, которая
  // у человека будет.
  if (error) {
    return (
      <span className={`text-fg-subtle ${line}`} title={formula}>
        {error}
      </span>
    );
  }

  return (
    <span className={`tabular-nums ${line}`} title={formula}>
      {typeof result === "number" ? result.toLocaleString(locale) : String(result ?? "")}
    </span>
  );
}

/**
 * Значение строки → операнд формулы.
 *
 * Число, пришедшее строкой, становится числом: колонки NUMBER бэкенд
 * отдаёт числами, а вот PICK_LIST и SINGLE_LINE с цифрами — строками,
 * и «12» * 2 у разборщика это #VALUE!.
 *
 * Незаполненное — ноль: иначе одна пустая ячейка в строке красит
 * ошибкой все формулы этой строки, а «ещё не заполнено» в арифметике
 * значит именно ноль.
 */
function toOperand(value: unknown): unknown {
  if (value === null || value === undefined || value === "") return 0;

  if (typeof value === "string") {
    const number = Number(value);
    return value.trim() !== "" && Number.isFinite(number) ? number : value;
  }

  return value;
}
