import { expect, test } from "vitest";
import { sqlGhost, sqlParams, sqlTablePrefix, tokenizeSql } from "./sql-syntax";

const kinds = (sql: string) =>
  tokenizeSql(sql)
    .filter((token) => token.text.trim())
    .map((token) => `${token.kind}:${token.text}`);

/*
 * Главное свойство: подложка с подсветкой лежит ровно под полем ввода,
 * и любой потерянный или удвоенный символ сдвигает текст под курсором.
 */
test("склейка лексем возвращает исходный текст до символа", () => {
  for (const sql of [
    "select * from orders where price > 100 -- заметка\n",
    "insert into t (a) values ('it''s', 2.5); /* блок */",
    "   \n\t",
    "",
  ]) {
    const tokens = tokenizeSql(sql);
    expect(tokens.map((token) => token.text).join("")).toBe(sql);
    expect(tokens.every((token) => sql.slice(token.start).startsWith(token.text))).toBe(true);
  }
});

test("слова из списка — ключевые, остальные имена — обычные", () => {
  expect(kinds("select id from orders")).toEqual([
    "keyword:select",
    "plain:id",
    "keyword:from",
    "plain:orders",
  ]);
});

test("звёздочка и скобки — знаки, число — число, строка — строка", () => {
  expect(kinds("count(*) = 1 and name = 'иван'")).toEqual([
    "keyword:count",
    "punct:(*)",
    "punct:=",
    "number:1",
    "keyword:and",
    "plain:name",
    "punct:=",
    "string:'иван'",
  ]);
});

/* Минус — оператор, два минуса — комментарий до конца строки. */
test("комментарии не распадаются на операторы", () => {
  expect(kinds("a - b -- хвост\nc")).toEqual([
    "plain:a",
    "punct:-",
    "plain:b",
    "comment:-- хвост",
    "plain:c",
  ]);
  expect(kinds("/* два\nряда */ a")).toEqual(["comment:/* два\nряда */", "plain:a"]);
});

/* Пока строку печатают, она незакрыта — это обычное состояние, а не сбой. */
test("незакрытая кавычка — уже строка", () => {
  expect(kinds("name = 'се")).toEqual(["plain:name", "punct:=", "string:'се"]);
  expect(sqlGhost("name = 'sel", 11)).toBe("");
});

test("подсказка дописывает начатое слово и держит регистр", () => {
  expect(sqlGhost("sel", 3)).toBe("ect");
  expect(sqlGhost("SEL", 3)).toBe("ECT");
  expect(sqlGhost("select * fr", 11)).toBe("om");
});

const TABLES = ["order_items", "orders", "users"];

test("после from подсказывается таблица, а не ключевое слово", () => {
  // «ord» подходит и под order by из списка слов, и под две таблицы:
  // берётся первая по алфавиту, а слово языка не берётся вовсе.
  expect(sqlGhost("select * from ord", 17, TABLES)).toBe("er_items");
  expect(sqlGhost("select * from ord", 17)).toBe("");
  expect(sqlGhost("insert into use", 15, TABLES)).toBe("rs");
  expect(sqlGhost("update ord", 10, TABLES)).toBe("er_items");
});

/* Вне табличного места список таблиц не при чём — там слова языка. */
test("в обычном месте подсказка берётся из ключевых слов", () => {
  expect(sqlGhost("select * from t where ord", 25, TABLES)).toBe("er");
});

/* Из двух подходящих слагов первый — всегда один и тот же. */
test("порядок слагов не зависит от того, как их отдал сервер", () => {
  expect(sqlGhost("from order", 10, ["orders", "order_items"])).toBe("_items");
  expect(sqlGhost("from order", 10, ["order_items", "orders"])).toBe("_items");
});

/* Запрос за таблицами уходит только там, где имя таблицы и ожидается. */
test("список таблиц спрашивается не на каждое слово", () => {
  expect(sqlTablePrefix("select * from ord", 17)).toBe("ord");
  expect(sqlTablePrefix("select * from o", 15)).toBe(""); // одна буква
  expect(sqlTablePrefix("select ord", 10)).toBe(""); // не табличное место
  expect(sqlTablePrefix("select * from ", 14)).toBe(""); // слово ещё не начато
});

test("подсказки нет там, где она была бы помехой", () => {
  expect(sqlGhost("s", 1)).toBe(""); // одна буква — подошла бы половина списка
  expect(sqlGhost("select", 6)).toBe(""); // слово уже целое
  expect(sqlGhost("orders", 6)).toBe(""); // не ключевое слово
  expect(sqlGhost("'sel", 4)).toBe(""); // внутри строки
  expect(sqlGhost("-- sel", 6)).toBe(""); // внутри комментария
  expect(sqlGhost("sel * from t", 3)).toBe(""); // дальше по строке уже есть текст
});

/*
 * Параметры ищутся ровно так же наивно, как на бэкенде: сравнивать
 * тут нужно не с «правильным» разбором SQL, а с тем, что подставит
 * `custom_endpoint.Run`.
 */
test("именованные параметры — те же, что найдёт бэкенд", () => {
  expect(sqlParams("select * from orders where id = :id and id = :id")).toEqual(["id"]);
  expect(sqlParams("select * from orders")).toEqual([]);
  expect(sqlParams("select id::text from t")).toEqual(["text"]); // промах бэкенда, и мы о нём предупреждаем
  expect(sqlParams("select * from t where at = '12:30'")).toEqual(["30"]); // его же
});
