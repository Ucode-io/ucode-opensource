/**
 * Ссылка-приглашение в проект.
 *
 * Приглашения как отдельной сущности в ручках нет: ни письма, ни записи
 * «приглашён». Есть регистрация по ссылке, в которой лежит, КУДА
 * записать пришедшего — проект, окружение, роль и тип клиента. Так это
 * и работает в старой админке (`components/InviteModal`): она собирает
 * тот же адрес и даёт его скопировать.
 *
 * Ключи именно такие, с дефисом у `project-id`: их читает наш экран
 * приглашения (`routes/invite.tsx`), а до него — старая админка.
 * Переименовать их значит сломать уже разосланные ссылки.
 *
 * Пустая строка вместо ссылки, если чего-то не хватает: ссылка без
 * идентификатора роли — это испорченное письмо, по которому человек
 * увидит «приглашение испорчено» и напишет обратно.
 */
export type Invitation = {
  origin: string;
  projectId: string;
  environmentId: string;
  roleId: string;
  clientTypeId: string;
};

export function inviteLink({
  origin,
  projectId,
  environmentId,
  roleId,
  clientTypeId,
}: Invitation): string {
  if (!origin || !projectId || !environmentId || !roleId || !clientTypeId) return "";

  const params = new URLSearchParams({
    "project-id": projectId,
    env_id: environmentId,
    role_id: roleId,
    client_type_id: clientTypeId,
  });

  // Хвостовая косая у origin — обычное дело для настроек; со ней адрес
  // получил бы двойной слэш и не совпал бы с маршрутом.
  return `${origin.replace(/\/+$/, "")}/invite?${params.toString()}`;
}
