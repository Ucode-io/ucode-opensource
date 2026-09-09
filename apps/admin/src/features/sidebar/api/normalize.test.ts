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
  // Внутри minio-папки лежат файлы, а не пункты меню: раскрывать нечего,
  // щелчок открывает хранилище.
  expect(kind("MINIO_FOLDER")).toBe("leaf");
  expect(kind("TABLE")).toBe("leaf");
  expect(kind("PIVOT")).toBe("leaf");
  expect(kind("REST")).toBe("leaf");
  expect(kind("LINK")).toBe("link");
  // Незнакомый тип не ломает сайдбар: показываем как обычный пункт.
  expect(kind("SOMETHING_NEW")).toBe("leaf");
});

test("наружу уводит link, а website_link встраивается рамкой", () => {
  // Две РАЗНЫЕ настройки на одном типе пункта: так их различала
  // и старая админка — сайт она открывала внутри админки, а ссылку
  // новой вкладкой.
  const site = toMenuNode({ id: "1", type: "LINK", attributes: { website_link: "https://a" } }, "en");
  expect(site.embedUrl).toBe("https://a");
  expect(site.href).toBeUndefined();

  const link = toMenuNode({ id: "2", type: "LINK", attributes: { link: "https://b" } }, "en");
  expect(link.href).toBe("https://b");
  expect(link.embedUrl).toBe("");

  expect(toMenuNode({ id: "3", type: "LINK" }, "en").href).toBeUndefined();
});

test("javascript: не встраивается и не открывается", () => {
  // Адрес приходит из поля, которое заполняет человек: это открытый ввод.
  const evil = { website_link: "javascript:alert(1)", link: "javascript:alert(1)" };
  const node = toMenuNode({ id: "1", type: "LINK", attributes: evil }, "en");

  expect(node.embedUrl).toBe("");
  expect(node.href).toBeUndefined();
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

test("микрофронтенд: пункт хранит идентификатор, а не адрес", () => {
  // Адрес сборки лежит у самого микрофронтенда (Function.url) и
  // дочитывается отдельно — см. features/microfrontend.
  const node = toMenuNode(
    { id: "1", label: "App", type: "MICROFRONTEND", microfrontend_id: "mf-1" },
    "ru",
  );

  expect(node.microfrontendId).toBe("mf-1");
  expect(toMenuNode({ id: "2", label: "T", type: "TABLE" }, "ru").microfrontendId).toBe("");
});

test("attributes.params раскладываются в карту", () => {
  // Форма пишет их массивом пар (MicrofrontendLinkModal.jsx:190),
  // а пользуются ими как набором именованных значений.
  const node = toMenuNode(
    {
      id: "1",
      label: "App",
      type: "MICROFRONTEND",
      attributes: {
        params: [
          { key: "mode", value: "compact" },
          { key: " tab ", value: "orders" },
          // Мусор из формы не должен ломать разбор целиком.
          { key: "", value: "x" },
          { value: "без ключа" },
          "строка вместо пары",
        ],
      },
    },
    "ru",
  );

  expect(node.params).toEqual({ mode: "compact", tab: "orders" });
});

test("params без списка — пустая карта, а не падение", () => {
  expect(toMenuNode({ id: "1", label: "A", type: "MICROFRONTEND" }, "ru").params).toEqual({});
  expect(
    toMenuNode({ id: "1", label: "A", type: "MICROFRONTEND", attributes: { params: "x" } }, "ru")
      .params,
  ).toEqual({});
});
