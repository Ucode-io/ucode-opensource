import { expect, test } from "vitest";
import { fileUrl, uploadFolder } from "./files";

test("путь из хранилища превращается в адрес, готовый — остаётся собой", () => {
  // VITE_CDN_URL в тестовом окружении оканчивается слэшем, путь — нет:
  // склейка не должна дать ни двойного слэша, ни склеенных имён.
  expect(fileUrl("media/1_photo.png")).toBe(`${import.meta.env.VITE_CDN_URL}media/1_photo.png`);
  expect(fileUrl("/media/1_photo.png")).toBe(`${import.meta.env.VITE_CDN_URL}media/1_photo.png`);
  expect(fileUrl("https://cdn.example.com/a.png")).toBe("https://cdn.example.com/a.png");
  expect(fileUrl("")).toBe("");
});

test("папку задаёт поле, пустую подменяем своей", () => {
  expect(uploadFolder({ path: "avatars" })).toBe("avatars");
  expect(uploadFolder({ path: "  " })).toBe("media");
  expect(uploadFolder({})).toBe("media");
});
