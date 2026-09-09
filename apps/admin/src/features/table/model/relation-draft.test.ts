import { expect, test } from "vitest";
import { toAutoFilters, toAutoFiltersBody } from "./relation-draft";

test("пары автофильтра читаются из связи", () => {
  const pairs = toAutoFilters({
    auto_filters: [{ field_to: "regions_id", field_from: "regions_id" }],
  });

  expect(pairs).toEqual([{ fieldFrom: "regions_id", fieldTo: "regions_id" }]);
});

test("недозаполненная пара условием не станет — ни туда, ни обратно", () => {
  expect(toAutoFilters({ auto_filters: [{ field_to: "regions_id" }] })).toEqual([]);
  expect(toAutoFilters({})).toEqual([]);
  expect(toAutoFilters({ auto_filters: null })).toEqual([]);

  expect(toAutoFiltersBody([{ fieldFrom: "", fieldTo: "regions_id" }])).toEqual([]);
});

test("в тело уезжают змеиные имена — их ждёт колонка", () => {
  expect(toAutoFiltersBody([{ fieldFrom: "a", fieldTo: "b" }])).toEqual([
    { field_from: "a", field_to: "b" },
  ]);
});
