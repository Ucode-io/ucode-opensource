import { useEffect, useState } from "react";
import { IconBuilding, IconShieldLock, IconUser, IconX } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { Icon } from "@/shared/ui/icon";
import { Modal } from "@/shared/ui/modal";
import { ProfileSettings } from "./ProfileSettings";
import { ProjectSettings } from "./ProjectSettings";
import { RoleSettings } from "./RoleSettings";

/**
 * Настройки — окно, а не страница.
 *
 * Их открывают из любого места и закрывают, вернувшись туда же: страница
 * означала бы, что человек уходит с таблицы, на которую потом должен
 * попасть обратно сам. В старой админке это тоже окно (SettingsPopup),
 * и там же лежит причина: настроек два десятка разделов, а работают
 * люди не в них.
 *
 * Разделов у нас три — профиль, проект и роли. Остальные (окружения,
 * ключи, функции, тарифы) появятся своими экранами; пустых пунктов
 * в списке нет: пункт, который ничего не открывает, — это обещание.
 */
const SECTIONS = [
  { id: "profile", labelKey: "settings.profile", icon: IconUser },
  { id: "project", labelKey: "settings.project", icon: IconBuilding },
  { id: "roles", labelKey: "settings.roles", icon: IconShieldLock },
] as const;

type SectionId = (typeof SECTIONS)[number]["id"];

export function SettingsDialog({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation();
  const [section, setSection] = useState<SectionId>("profile");

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => event.key === "Escape" && onClose();
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  return (
    <Modal onClose={onClose}>
      {/* Окно широкое: в правах помещается матрица «таблица × право»,
          а она не сжимается — колонок одиннадцать. На узком экране
          растягивается до его краёв. */}
      <div className="flex h-[min(48rem,92vh)] w-full max-w-[min(84rem,96vw)] overflow-hidden rounded-xl border border-border bg-surface shadow-modal">
        {/* Список разделов слева: их немного, и прятать их в выпадающий
            список значит заставлять открывать его на каждый переход. */}
        <nav className="flex w-52 shrink-0 flex-col gap-0.5 border-r border-border bg-bg p-2">
          <span className="px-2 py-1 text-2xs font-medium tracking-wide text-fg-subtle uppercase">
            {t("workspace.settings")}
          </span>

          {SECTIONS.map((item) => (
            <button
              key={item.id}
              type="button"
              onClick={() => setSection(item.id)}
              className={`flex h-8 items-center gap-2 rounded-md px-2 text-left text-sm transition-colors ${
                section === item.id
                  ? "bg-surface text-fg shadow-raised"
                  : "text-fg-muted hover:bg-surface-hover hover:text-fg"
              }`}
            >
              <Icon as={item.icon} size={16} className="shrink-0" />
              {t(item.labelKey)}
            </button>
          ))}
        </nav>

        <div className="flex min-w-0 flex-1 flex-col">
          <header className="flex h-header shrink-0 items-center gap-2 border-b border-border px-4">
            <span className="flex-1 text-sm font-medium">
              {t(SECTIONS.find((item) => item.id === section)?.labelKey ?? "workspace.settings")}
            </span>

            <button
              type="button"
              onClick={onClose}
              aria-label={t("action.close")}
              title={t("action.close")}
              className="grid size-7 shrink-0 place-items-center rounded-md text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
            >
              <Icon as={IconX} size={16} />
            </button>
          </header>

          {/* Роли занимают всю площадь и прокручиваются сами: это
              широкая матрица, а не столбик полей, и общий отступ
              с прокруткой ей только мешают. */}
          {section === "roles" ? (
            <RoleSettings />
          ) : (
            <div className="min-h-0 flex-1 overflow-y-auto p-6">
              {section === "profile" ? <ProfileSettings /> : <ProjectSettings />}
            </div>
          )}
        </div>
      </div>
    </Modal>
  );
}
