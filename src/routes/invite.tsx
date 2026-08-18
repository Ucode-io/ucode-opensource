import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import { AuthLayout, InviteForm } from "@/features/auth";

/**
 * Приглашение в проект. Всё нужное лежит в адресе — ссылку присылает
 * тот, кто приглашает.
 *
 * Ключи именно такие, с дефисом в `project-id`: их пишет приглашающая
 * сторона, и переименование сломало бы уже разосланные письма.
 *
 * `beforeLoad` с проверкой сессии здесь НЕТ, в отличие от входа:
 * по приглашению приходят и с открытой чужой сессией в этой же вкладке,
 * и уводить такого человека на его старый проект — значит не дать
 * принять приглашение вовсе.
 */
const searchSchema = z.object({
  "project-id": z.string().catch(""),
  env_id: z.string().catch(""),
  role_id: z.string().catch(""),
  client_type_id: z.string().catch(""),
});

export const Route = createFileRoute("/invite")({
  validateSearch: searchSchema,
  component: InvitePage,
});

function InvitePage() {
  const navigate = useNavigate();
  const search = Route.useSearch();
  const { t } = useTranslation();

  const invite = {
    projectId: search["project-id"],
    environmentId: search.env_id,
    roleId: search.role_id,
    clientTypeId: search.client_type_id,
  };

  // Ссылка без идентификаторов — это испорченное письмо, а не форма
  // регистрации: заводить пользователя некуда.
  const complete = Object.values(invite).every(Boolean);

  return (
    <AuthLayout>
      {complete ? (
        <InviteForm invite={invite} onSuccess={() => void navigate({ to: "/" })} />
      ) : (
        <p className="max-w-sm rounded-md bg-danger-subtle px-3 py-2 text-sm text-danger">
          {t("auth.inviteBroken")}
        </p>
      )}
    </AuthLayout>
  );
}
