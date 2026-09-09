import { expect, test } from "vitest";
import { menuUpdateBody } from "./mutations";
import { toMenuNode } from "./normalize";

const table = toMenuNode(
  {
    id: "m1",
    label: "Orders",
    icon: "tabler:table",
    type: "TABLE",
    parent_id: "p1",
    table_id: "t1",
    layout_id: "l1",
    attributes: { label_en: "Orders", label_cyr: "Заказы" },
  },
  "cyr",
);

test("переименование сохраняет всё, что PUT перезаписал бы пустотой", () => {
  // SQL пишет строку целиком: пропущенный type — отказ, пропущенный
  // table_id — таблица, отвязанная от своего пункта меню.
  const body = menuUpdateBody(table, { labels: { en: "Requests", cyr: "Заявки" } }, "proj");

  expect(body).toMatchObject({
    id: "m1",
    // Колонка label — первое непустое имя: пустой она быть не может.
    label: "Requests",
    type: "TABLE",
    table_id: "t1",
    layout_id: "l1",
    parent_id: "p1",
    project_id: "proj",
  });
});

test("перенос в другую папку не подменяет базовую подпись локализованной", () => {
  const body = menuUpdateBody(table, { parentId: "p2" }, "proj");

  expect(body.parent_id).toBe("p2");
  expect(body.label).toBe("Orders");
});

test("attributes дополняются, а не заменяются", () => {
  // Иначе перенос ссылки стирает её адрес, а переименование — подписи
  // по языкам: бэкенд кладёт в колонку ровно то, что пришло.
  const body = menuUpdateBody(table, { attributes: { link: "https://a" } }, "proj");

  expect(body.attributes).toEqual({
    label_en: "Orders",
    label_cyr: "Заказы",
    link: "https://a",
  });
});
