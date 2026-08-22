import { useState } from "react";
import { IconPlus } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
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
  const [type, setType] = useState<CreatableType | null>(null);
  /** Открыт выбор шаблона: готовый набор таблиц разворачивается целиком. */
  const [templates, setTemplates] = useState(false);
  const create = useCreateMenu();

  const submit = (value: MenuFormValue) => {
    if (!type) return;
    create.mutate(
      {
        labels: value.labels,
        icon: value.icon,
        type,
        parentId,
        ...(type === "TABLE" ? { slug: value.slug } : {}),
        ...(type === "MICROFRONTEND" ? { microfrontendId: value.microfrontendId } : {}),
        attributes: menuAttributes(type, value),
      },
      { onSuccess: () => setType(null) },
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
                setType("TABLE");
              }}
            >
              {t("menuAction.createTable")}
            </PopoverItem>
            <PopoverItem
              onClick={() => {
                close();
                setType("FOLDER");
              }}
            >
              {t("menuAction.createFolder")}
            </PopoverItem>
            <PopoverItem
              onClick={() => {
                close();
                setType("MINIO_FOLDER");
              }}
            >
              {t("menuAction.createFiles")}
            </PopoverItem>
            <PopoverItem
              onClick={() => {
                close();
                setType("LINK");
              }}
            >
              {t("menuAction.createLink")}
            </PopoverItem>
            <PopoverItem
              onClick={() => {
                close();
                setType("MICROFRONTEND");
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

      {type && (
        <MenuFormDialog
          title={t(CREATE_TITLES[type])}
          initial={EMPTY_MENU_FORM}
          type={type}
          needsSlug={type === "TABLE"}
          needsRemote={type === "MICROFRONTEND"}
          busy={create.isPending}
          onSubmit={submit}
          onClose={() => setType(null)}
        />
      )}
    </>
  );
}
