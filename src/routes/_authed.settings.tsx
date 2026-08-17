import { createFileRoute } from "@tanstack/react-router";
import { SidebarToggleButton } from "@/features/sidebar";
import { useTranslation } from "react-i18next";

/** Настройки. Наполнение — пункт 9 плана: Account, ProjectSettings, роли. */
export const Route = createFileRoute("/_authed/settings")({ component: SettingsPage });

function SettingsPage() {
  const { t } = useTranslation();

  return (
    <div className="flex h-full flex-col">
      <header className="flex h-11 items-center gap-2 border-b border-border px-4">
        <SidebarToggleButton />
        <span className="text-sm font-medium">{t("workspace.settings")}</span>
      </header>
      <div className="grid flex-1 place-items-center p-8">
        <p className="text-sm text-fg-muted">{t("menu.placeholder")}</p>
      </div>
    </div>
  );
}
