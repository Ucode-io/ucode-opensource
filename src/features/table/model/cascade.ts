import type { Relation } from "./types";

/**
 * Каскадные списки: выбор в одном списке сужает следующий.
 *
 * Область → город → район. Связь ведёт на «районы», но выбирать район
 * среди всех районов страны бессмысленно, поэтому выбор идёт сверху
 * вниз: сперва область, в ней город, в нём район.
 *
 * **Формат чужой и читается, а не придумывается.** Колонка
 * `relation.cascadings` (`000001_init_tables.up.sql:114`) — массив
 * звеньев `{table_slug, field_slug}`, и лежит он БЛИЖНИМ КОНЦОМ
 * ВПЕРЁД:
 *
 *   [0] {districts, district_id}  куда ведёт связь и наша колонка
 *   [1] {cities,    city_id}      родитель района и колонка В РАЙОНЕ
 *   [2] {regions,   region_id}    родитель города и колонка В ГОРОДЕ
 *
 * То есть у звена `{table_slug: T, field_slug: F}` таблица — это T,
 * а поле — колонка в таблице УРОВНЕМ НИЖЕ, которая на T ссылается.
 * Пара разъезжается на один шаг, и это единственное, что в формате
 * неочевидно.
 *
 * Показывается всё это в обратном порядке — от дальнего предка
 * к цели, — поэтому наружу отдаётся уже развёрнутый список шагов:
 * `steps[i]` — что показать на шаге `i`, `steps[i - 1].fieldSlug` —
 * чем отобрать. Так же ходит и старая админка
 * (`views/views/components/ElementGenerators/CascadingItem.jsx:98`),
 * только считает индексы от конца на каждом шаге.
 */
export type CascadeStep = {
  /** Таблица, строки которой показываются на этом шаге. */
  tableSlug: string;
  /**
   * Колонка в таблице СЛЕДУЮЩЕГО шага, ссылающаяся на эту. Ею и
   * отбирается следующий список. У последнего шага не нужна.
   */
  fieldSlug: string;
};

/** Звено, как оно лежит в базе. Наружу не отдаётся — только шаги. */
type Link = { table_slug?: unknown; field_slug?: unknown };

function links(relation: Relation): Link[] {
  const raw = relation.raw["cascadings"];
  return Array.isArray(raw) ? (raw as Link[]) : [];
}

/**
 * Шаги каскада — от дальнего предка к цели. Пусто — каскада нет,
 * и связь выбирается одним списком.
 *
 * Одного звена мало: это просто «куда ведёт связь», без единого шага
 * сужения. Так же считает и старая админка (`cascadings.length > 1`).
 *
 * Последний шаг обязан совпасть с целью связи: цепочка, собранная
 * для прежней таблицы, после смены цели ведёт не туда, и молча
 * применить её значило бы записать в поле чужой guid.
 */
export function cascadeSteps(relation: Relation): CascadeStep[] {
  const chain = links(relation);
  if (chain.length < 2) return [];

  const steps: CascadeStep[] = [];

  for (let i = chain.length - 1; i >= 0; i--) {
    const link = chain[i];
    const tableSlug = typeof link?.table_slug === "string" ? link.table_slug : "";
    const fieldSlug = typeof link?.field_slug === "string" ? link.field_slug : "";
    if (!tableSlug || !fieldSlug) return [];

    steps.push({ tableSlug, fieldSlug });
  }

  return steps[steps.length - 1]?.tableSlug === relation.toSlug ? steps : [];
}

/**
 * Цепочка → то, что уезжает в колонку. Обратная к `cascadeSteps`:
 * форма собирает шаги в человеческом порядке, база хранит в своём.
 */
export function toCascadingsBody(steps: CascadeStep[]): Link[] {
  if (steps.length < 2) return [];

  return steps
    .map((step) => ({ table_slug: step.tableSlug, field_slug: step.fieldSlug }))
    .reverse();
}
