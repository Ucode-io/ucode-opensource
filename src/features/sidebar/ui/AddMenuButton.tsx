import { useState } from "react";
import { IconPlus } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { Icon } from "@/shared/ui/icon";
import { Popover, PopoverItem } from "@/shared/ui/popover";
import { useCreateMenu } from "../api/mutations";
import { MenuFormDialog, type MenuFormValue } from "./MenuFormDialog";

/** Кнопка «+» в шапке сайдбара: создаёт пункт на верхнем уровне. */
export function AddMenuButton({ parentId }: { parentId: string }) {
  const { t } = useTranslation();
  const [type, setType] = useState<"FOLDER" | "TABLE" | "LINK" | null>(null);
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
        ...(type === "LINK" ? { attributes: { link: value.href } } : {}),
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
                setType("LINK");
              }}
            >
              {t("menuAction.createLink")}
            </PopoverItem>
          </>
        )}
      </Popover>

      {type && (
        <MenuFormDialog
          title={t(
            type === "FOLDER"
              ? "menuForm.createFolder"
              : type === "LINK"
                ? "menuForm.createLink"
                : "menuForm.createTable",
          )}
          initial={{ labels: {}, icon: "", href: "", slug: "" }}
          type={type}
          needsSlug={type === "TABLE"}
          busy={create.isPending}
          onSubmit={submit}
          onClose={() => setType(null)}
        />
      )}
    </>
  );
}
