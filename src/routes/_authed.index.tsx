import { createFileRoute } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";

export const Route = createFileRoute("/_authed/")({ component: Home });

/**
 * Временная страница. Уйдёт, когда появится экран по умолчанию —
 * скорее всего, первый пункт меню.
 *
 * Переключатели темы и языка отсюда убраны: они переехали в поповер
 * рабочего пространства, где до них можно дотянуться с любого экрана.
 */
function Home() {
  const { t } = useTranslation();

  return (
    <div className="grid h-full place-items-center p-8">
      <p className="text-sm text-fg-muted">{t("menu.placeholder")}</p>
    </div>
  );
}
