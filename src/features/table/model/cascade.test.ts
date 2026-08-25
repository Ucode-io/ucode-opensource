import { expect, test } from "vitest";
import { cascadeSteps, toCascadingsBody } from "./cascade";
import type { Relation } from "./types";

/** Область → город → район: цепочка лежит ближним концом вперёд. */
const CHAIN = [
  { table_slug: "districts", field_slug: "district_id" },
  { table_slug: "cities", field_slug: "city_id" },
  { table_slug: "regions", field_slug: "region_id" },
];

const relation = (raw: Record<string, unknown>, toSlug = "districts") =>
  ({ toSlug, raw }) as Relation;

test("шаги разворачивают цепочку: от дальнего предка к цели", () => {
  expect(cascadeSteps(relation({ cascadings: CHAIN }))).toEqual([
    { tableSlug: "regions", fieldSlug: "region_id" },
    { tableSlug: "cities", fieldSlug: "city_id" },
    { tableSlug: "districts", fieldSlug: "district_id" },
  ]);
});

test("каскада нет: пусто, одно звено, чужой формат", () => {
  // Одно звено — это «куда ведёт связь», без единого шага сужения.
  expect(cascadeSteps(relation({ cascadings: [CHAIN[0]] }))).toEqual([]);
  expect(cascadeSteps(relation({ cascadings: [] }))).toEqual([]);
  expect(cascadeSteps(relation({}))).toEqual([]);
  expect(cascadeSteps(relation({ cascadings: "regions" }))).toEqual([]);

  // Звено без половины пары применить нечем.
  expect(cascadeSteps(relation({ cascadings: [CHAIN[0], { table_slug: "cities" }] }))).toEqual([]);
});

test("цепочка не от этой связи не применяется", () => {
  /*
   * Цель связи сменили, а цепочку собирали для прежней: последний шаг
   * ведёт не туда, и молчаливое применение записало бы в поле guid
   * чужой таблицы.
   */
  expect(cascadeSteps(relation({ cascadings: CHAIN }, "streets"))).toEqual([]);
});

test("шаги и тело запроса — обратные друг другу", () => {
  const steps = cascadeSteps(relation({ cascadings: CHAIN }));

  expect(toCascadingsBody(steps)).toEqual(CHAIN);
  // Одного шага мало: цепочки из него не выйдет, уезжает пустота.
  expect(toCascadingsBody(steps.slice(0, 1))).toEqual([]);
});
