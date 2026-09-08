/**
 * Какие колонки живут в DOM.
 *
 * Строк в таблице тысячи, но и колонок бывает полсотни: на каждой
 * видимой строке это полсотни ячеек, из которых на экран попадает
 * восемь. Виртуализация колонок считает то же окно, что и по строкам,
 * только вбок.
 *
 * Закреплённые колонки в окно не входят и рисуются всегда: они липкие
 * и видны при любой прокрутке — заменить их распоркой значит убрать
 * с экрана то, что человек как раз и закрепил, чтобы видеть.
 */
export type ColumnWindow = {
  /** Сколько колонок закреплено слева. Они всегда в начале ряда. */
  pinned: number;
  /** Первая и следующая за последней из прокручиваемых колонок. */
  from: number;
  to: number;
  /** Сколько колонок заменено распоркой слева и справа от окна. */
  before: number;
  after: number;
};

/**
 * @param total сколько всего колонок
 * @param pinned сколько из них закреплено слева
 * @param visible индексы видимых колонок от виртуализатора; пусто —
 *        виртуализация выключена или ещё не считала, и тогда окно
 *        охватывает всё: лучше лишние ячейки, чем пустая таблица
 */
export function columnWindow(
  total: number,
  pinned: number,
  visible: readonly number[],
): ColumnWindow {
  const first = visible[0];
  const last = visible[visible.length - 1];

  if (first === undefined || last === undefined) {
    return { pinned, from: pinned, to: total, before: 0, after: 0 };
  }

  // Окно начинается там, где кончаются закреплённые: их куски виртуализатор
  // тоже отдаёт, но рисуются они отдельно, и второй раз им в ряду нечего делать.
  const from = Math.min(Math.max(first, pinned), total);
  const to = Math.min(Math.max(last + 1, from), total);

  return { pinned, from, to, before: from - pinned, after: total - to };
}
