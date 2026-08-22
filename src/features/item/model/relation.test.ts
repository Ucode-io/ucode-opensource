import { expect, test } from "vitest";
import type { Relation } from "@/features/table";
import { autoFilterValues } from "./relation";

/** Связь с настроенным автофильтром: город отбирается по региону строки. */
function relationWith(autoFilters: unknown): Relation {
  return {
    id: "r1",
    type: "Many2One",
    toSlug: "cities",
    toLabel: "Города",
    toLabels: {},
    title: "",
    fieldFrom: "cities_id",
    direction: "outgoing",
    linkField: "cities_id",
    viewFieldSlugs: ["name"],
    viewFieldIds: [],
    raw: { auto_filters: autoFilters },
  };
}

const PAIRS = [{ field_from: "regions_id", field_to: "regions_id" }];

test("значение поля-источника уходит условием по чужой колонке", () => {
  const values = autoFilterValues(relationWith(PAIRS), { regions_id: "reg-1" });

  expect(values).toEqual({ regions_id: "reg-1" });
});

test("незаполненный источник условием не становится", () => {
  // Иначе список городов был бы пуст до выбора региона — а пустой
  // список читается как «связывать не с чем».
  expect(autoFilterValues(relationWith(PAIRS), {})).toEqual({});
  expect(autoFilterValues(relationWith(PAIRS), { regions_id: "" })).toEqual({});
  expect(autoFilterValues(relationWith(PAIRS), { regions_id: null })).toEqual({});
});

test("пары действуют вместе", () => {
  const values = autoFilterValues(
    relationWith([
      { field_from: "regions_id", field_to: "regions_id" },
      { field_from: "kind", field_to: "kind" },
    ]),
    { regions_id: "reg-1", kind: "big", extra: "не наше дело" },
  );

  expect(values).toEqual({ regions_id: "reg-1", kind: "big" });
});

test("ненастроенная связь условий не добавляет", () => {
  expect(autoFilterValues(relationWith(null), { regions_id: "reg-1" })).toEqual({});
  expect(autoFilterValues(relationWith([{}]), { regions_id: "reg-1" })).toEqual({});
  expect(autoFilterValues(relationWith([{ field_from: "regions_id" }]), {})).toEqual({});
});
