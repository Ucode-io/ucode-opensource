import { useState } from "react";
import { IconPlus } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { useGlobalRight } from "@/features/auth";
import { Icon } from "@/shared/ui/icon";
import { Popover, PopoverItem } from "@/shared/ui/popover";
import { TemplateDialog } from "@/features/templates";
import { useCreateMenu } from "../api/mutations";
import {
  CREATE_TITLES,
  EMPTY_MENU_FORM,
  MenuFormDialog,
  menuAttributes,
  type CreatableType,
  type MenuFormValue,
} from "./MenuFormDialog";

/** Кнопка «+» в шапке сайдбара: создаёт пункт на верхнем уровне. */
export function AddMenuButton({ parentId }: { parentId: string }) {
  const { t } = useTranslation();
  /*
   * Заводить пункты верхнего уровня разрешает глобальное право роли —
   * `menu_button` (LayoutSidebar/index.jsx:612). Внутри папки решает
   * право на саму папку (`can.write`), а у корня своей строки нет,
   * и спросить о нём больше некого.
   */
  const canCreate = useGlobalRight("menu_button");
  /** Что заводим. `existing` — пункт на уже существующую таблицу. */
  const [form, setForm] = useState<{ type: CreatableType; existing?: boolean } | null>(null);
  /** Открыт выбор шаблона: готовый набор таблиц разворачивается целиком. */
  const [templates, setTemplates] = useState(false);
  const create = useCreateMenu();

  if (!canCreate) return null;

  const submit = (value: MenuFormValue) => {
    if (!form) return;
    const { type, existing } = form;

    create.mutate(
      {
        labels: value.labels,
        icon: value.icon,
        type,
        parentId,
        ...(type === "TABLE" && !existing ? { slug: value.slug } : {}),
        ...(existing ? { tableId: value.tableId } : {}),
        ...(type === "MICROFRONTEND" ? { microfrontendId: value.microfrontendId } : {}),
        attributes: menuAttributes(type, value),
      },
      { onSuccess: () => setForm(null) },
    );
  };

  return (
    <>
      <Popover
        align="end"
        trigger={({ open, toggle }) => (
          <button
            type="button"
            onClick={toggle}
            aria-label={t("sidebar.add")}
            className={`grid size-6 shrink-0 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-surface-hover hover:text-fg ${
              open ? "bg-surface-active text-fg" : ""
            }`}
          >
            <Icon as={IconPlus} size={14} />
          </button>
        )}
      >
        {(close) => (
          <>
            <PopoverItem
              onClick={() => {
                close();
                setForm({ type: "TABLE" });
              }}
            >
              {t("menuAction.createTable")}
            </PopoverItem>
            <PopoverItem
              onClick={() => {
                close();
                setForm({ type: "TABLE", existing: true });
              }}
            >
              {t("menuAction.linkTable")}
            </PopoverItem>
            <PopoverItem
              onClick={() => {
                close();
                setForm({ type: "FOLDER" });
              }}
            >
              {t("menuAction.createFolder")}
            </PopoverItem>
            <PopoverItem
              onClick={() => {
                close();
                setForm({ type: "MINIO_FOLDER" });
              }}
            >
              {t("menuAction.createFiles")}
            </PopoverItem>
            <PopoverItem
              onClick={() => {
                close();
                setForm({ type: "LINK" });
              }}
            >
              {t("menuAction.createLink")}
            </PopoverItem>
            <PopoverItem
              onClick={() => {
                close();
                setForm({ type: "MICROFRONTEND" });
              }}
            >
              {t("menuAction.createMicrofrontend")}
            </PopoverItem>
            <PopoverItem
              onClick={() => {
                close();
                setTemplates(true);
              }}
            >
              {t("menuAction.fromTemplate")}
            </PopoverItem>
          </>
        )}
      </Popover>

      {templates && <TemplateDialog onClose={() => setTemplates(false)} />}

      {form && (
        <MenuFormDialog
          title={t(form.existing ? "menuForm.linkTable" : CREATE_TITLES[form.type])}
          initial={EMPTY_MENU_FORM}
          type={form.type}
          needsSlug={form.type === "TABLE" && !form.existing}
          needsTable={Boolean(form.existing)}
          needsRemote={form.type === "MICROFRONTEND"}
          busy={create.isPending}
          onSubmit={submit}
          onClose={() => setForm(null)}
        />
      )}
    </>
  );
}
