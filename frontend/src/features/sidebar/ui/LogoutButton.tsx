import { useState } from "react";
import { IconLogout } from "@tabler/icons-react";
import { useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { logout } from "@/features/auth";
import { Icon } from "@/shared/ui/icon";
import { PopoverItem } from "@/shared/ui/popover";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";

/**
 * Выход со спросом. Пункт стоит вплотную к безобидным действиям,
 * а промах по нему стоит повторного ввода пароля.
 */
export function LogoutButton({ onDone }: { onDone: () => void }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [asking, setAsking] = useState(false);

  return (
    <>
      <PopoverItem danger icon={<Icon as={IconLogout} size={14} />} onClick={() => setAsking(true)}>
        {t("auth.signOut")}
      </PopoverItem>

      {asking && (
        <ConfirmDialog
          title={t("auth.signOutTitle")}
          description={t("auth.signOutDescription")}
          confirmLabel={t("auth.signOut")}
          busy={false}
          onConfirm={() => {
            setAsking(false);
            onDone();
            logout();
            void navigate({ to: "/login" });
          }}
          onClose={() => setAsking(false)}
        />
      )}
    </>
  );
}
