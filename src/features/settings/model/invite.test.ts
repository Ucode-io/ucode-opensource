import { expect, test } from "vitest";
import { inviteLink } from "./invite";

const full = {
  origin: "https://admin.ucode.io",
  projectId: "p1",
  environmentId: "e1",
  roleId: "r1",
  clientTypeId: "c1",
};

test("ссылка ведёт на наш экран приглашения и несёт все четыре ключа", () => {
  expect(inviteLink(full)).toBe(
    "https://admin.ucode.io/invite?project-id=p1&env_id=e1&role_id=r1&client_type_id=c1",
  );
});

test("хвостовая косая в адресе не даёт двойного слэша", () => {
  expect(inviteLink({ ...full, origin: "https://admin.ucode.io/" })).toContain(".io/invite?");
});

/*
 * Ссылка без роли открывается словами «приглашение испорчено»
 * (routes/invite.tsx): лучше не дать её скопировать вовсе.
 */
test("неполные данные — ссылки нет", () => {
  expect(inviteLink({ ...full, roleId: "" })).toBe("");
  expect(inviteLink({ ...full, environmentId: "" })).toBe("");
});
