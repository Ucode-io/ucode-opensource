import { expect, test } from "vitest";
import { toMenuNode } from "./normalize";

test("подпись берётся из attributes.label_<язык>, иначе из label", () => {
  const dto = { id: "1", label: "Orders", type: "TABLE", attributes: { label_ru: "Заказы" } };

  expect(toMenuNode(dto, "ru").label).toBe("Заказы");
  expect(toMenuNode(dto, "en").label).toBe("Orders");
});

test("пустая подпись не оставляет строку без текста", () => {
  expect(toMenuNode({ id: "1", label: "   ", type: "TABLE" }, "en").label).toBe("—");
});

test("поведение выводится из типа, а не задаётся бэкендом", () => {
  const kind = (type: string) => toMenuNode({ id: "1", type }, "en").kind;

  expect(kind("FOLDER")).toBe("group");
  expect(kind("WIKI_FOLDER")).toBe("group");
  expect(kind("MINIO_FOLDER")).toBe("group");
  expect(kind("TABLE")).toBe("leaf");
  expect(kind("PIVOT")).toBe("leaf");
  expect(kind("REST")).toBe("leaf");
  expect(kind("LINK")).toBe("link");
  // Незнакомый тип не ломает сайдбар: показываем как обычный пункт.
  expect(kind("SOMETHING_NEW")).toBe("leaf");
});

test("ссылка берёт адрес из website_link или link", () => {
  expect(toMenuNode({ id: "1", type: "LINK", attributes: { website_link: "https://a" } }, "en").href).toBe("https://a");
  expect(toMenuNode({ id: "2", type: "LINK", attributes: { link: "https://b" } }, "en").href).toBe("https://b");
  expect(toMenuNode({ id: "3", type: "LINK" }, "en").href).toBeUndefined();
});

test("позиция берётся из ответа, а не выдумывается клиентом", () => {
  // Сервер уже отсортировал по "order", но самого поля в ответе нет.
  // Любая своя сортировка сломает порядок — здесь только индекс.
  expect(toMenuNode({ id: "1", label: "Яблоко", type: "TABLE" }, "ru", 0).order).toBe(0);
  expect(toMenuNode({ id: "2", label: "Абрикос", type: "TABLE" }, "ru", 1).order).toBe(1);
});

test("подпись пункта: сначала локаль интерфейса, потом язык данных", () => {
  const dto = {
    id: "1",
    label: "Orders",
    type: "TABLE",
    attributes: { label_ru: "Заказы", label_cyr: "Буюртмалар" },
  };

  // Переключение языка интерфейса меняет надпись в сайдбаре — ради этого
  // локаль и стоит первой.
  expect(toMenuNode(dto, ["ru", "cyr"]).label).toBe("Заказы");

  // Локали интерфейса среди языков проекта нет (проект на en+cyr) —
  // остаётся язык данных, а не голая колонка label.
  expect(toMenuNode(dto, ["uz", "cyr"]).label).toBe("Буюртмалар");

  // Не заполнено ни на одном — базовая колонка.
  expect(toMenuNode(dto, ["uz", "kk"]).label).toBe("Orders");
});
