import { useState } from "react";
import { IconDots } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { Icon } from "@/shared/ui/icon";
import { Popover, PopoverItem, PopoverSeparator } from "@/shared/ui/popover";
import { useUi } from "@/shared/lib/ui-store";
import { useCreateMenu, useDeleteMenu, useUpdateMenu } from "../api/mutations";
import { actionsFor, typeWordKey, type MenuActionId } from "../model/actions";
import type { MenuNode } from "../model/types";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import { MenuFormDialog, type MenuFormValue } from "./MenuFormDialog";

type CreatableType = "FOLDER" | "TABLE" | "LINK";

type Dialog =
  | { kind: "edit" }
  | { kind: "create"; type: CreatableType }
  | { kind: "delete" }
  | null;

/** Заголовок окна создания зависит только от типа. */
const CREATE_TITLES: Record<CreatableType, "menuForm.createFolder" | "menuForm.createTable" | "menuForm.createLink"> = {
  FOLDER: "menuForm.createFolder",
  TABLE: "menuForm.createTable",
  LINK: "menuForm.createLink",
};

/**
 * Кнопка «⋮» на строке меню. Набор пунктов собирается из реестра действий
 * и прав самого пункта — ветвлений по типу здесь нет.
 */
export function MenuRowActions({ node, isAdmin }: { node: MenuNode; isAdmin: boolean }) {
  const { t } = useTranslation();
  const [dialog, setDialog] = useState<Dialog>(null);

  const create = useCreateMenu();
  const update = useUpdateMenu();
  const remove = useDeleteMenu();
  const forgetMenu = useUi((state) => state.forgetMenu);

  const actions = actionsFor(node, isAdmin);
  if (actions.length === 0) return null;

  const typeWord = t(typeWordKey(node.type));

  const run = (id: MenuActionId, close: () => void) => {
    close();
    if (id === "edit") setDialog({ kind: "edit" });
    if (id === "create-folder") setDialog({ kind: "create", type: "FOLDER" });
    if (id === "create-table") setDialog({ kind: "create", type: "TABLE" });
    if (id === "create-link") setDialog({ kind: "create", type: "LINK" });
    if (id === "delete") setDialog({ kind: "delete" });
    // settings и make-template подключаются вместе со своими экранами.
  };

  const submitForm = (value: MenuFormValue) => {
    if (dialog?.kind === "edit") {
      update.mutate(
        { node, labels: value.labels, icon: value.icon, ...linkAttributes(node.type, value) },
        { onSuccess: () => setDialog(null) },
      );
    } else if (dialog?.kind === "create") {
      create.mutate(
        {
          labels: value.labels,
          icon: value.icon,
          type: dialog.type,
          parentId: node.id,
          ...(dialog.type === "TABLE" ? { slug: value.slug } : {}),
          ...linkAttributes(dialog.type, value),
        },
        { onSuccess: () => setDialog(null) },
      );
    }
  };

  return (
    <>
      <Popover
        align="end"
        trigger={({ open, toggle }) => (
          <button
            type="button"
            aria-label={t("menuAction.more")}
            onClick={(event) => {
              event.preventDefault();
              event.stopPropagation();
              toggle();
            }}
            className={`grid size-6 shrink-0 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-surface-active hover:text-fg ${
              open ? "bg-surface-active text-fg" : "opacity-0 group-hover/row:opacity-100"
            }`}
          >
            <Icon as={IconDots} size={14} />
          </button>
        )}
      >
        {(close) =>
          actions.map((action) => (
            <div key={action.id}>
              {action.separated && <PopoverSeparator />}
              <PopoverItem danger={action.danger ?? false} onClick={() => run(action.id, close)}>
                {t(action.labelKey, { type: typeWord })}
              </PopoverItem>
            </div>
          ))
        }
      </Popover>

      {dialog?.kind === "edit" && (
        <MenuFormDialog
          title={t("menuForm.editTitle", { type: typeWord })}
          initial={{ labels: node.labels, icon: node.icon, href: node.href ?? "", slug: "" }}
          type={node.type}
          busy={update.isPending}
          onSubmit={submitForm}
          onClose={() => setDialog(null)}
        />
      )}

      {dialog?.kind === "create" && (
        <MenuFormDialog
          title={t(CREATE_TITLES[dialog.type])}
          initial={{ labels: {}, icon: "", href: "", slug: "" }}
          type={dialog.type}
          needsSlug={dialog.type === "TABLE"}
          busy={create.isPending}
          onSubmit={submitForm}
          onClose={() => setDialog(null)}
        />
      )}

      {dialog?.kind === "delete" && (
        <ConfirmDialog
          title={t("menuForm.deleteTitle", { label: node.label })}
          description={t("menuForm.deleteDescription")}
          confirmLabel={t("action.delete")}
          busy={remove.isPending}
          onConfirm={() =>
            remove.mutate(
              { id: node.id },
              {
                onSuccess: () => {
                  // Раскрытие помнится по id: у удалённого пункта ему
                  // больше нечему отвечать, и в памяти он остался бы навсегда.
                  forgetMenu(node.id);
                  setDialog(null);
                },
              },
            )
          }
          onClose={() => setDialog(null)}
        />
      )}
    </>
  );
}

/** Адрес ссылки живёт в attributes.link — у остальных типов его нет. */
function linkAttributes(type: string, value: MenuFormValue) {
  return type === "LINK" ? { attributes: { link: value.href } } : {};
}
