import { useState } from "react";
import { IconDots } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { useIsSuperRole } from "@/features/auth";
import { Icon } from "@/shared/ui/icon";
import { Popover, PopoverItem, PopoverSeparator } from "@/shared/ui/popover";
import { TemplateCreateDialog } from "@/features/templates";
import { useUi } from "@/shared/lib/ui-store";
import { useCreateMenu, useDeleteMenu, useUpdateMenu } from "../api/mutations";
import { actionsFor, typeWordKey, type MenuActionId } from "../model/actions";
import type { MenuNode } from "../model/types";
import { ConfirmDialog } from "@/shared/ui/confirm-dialog";
import {
  CREATE_TITLES,
  EMPTY_MENU_FORM,
  MenuFormDialog,
  menuAttributes,
  type CreatableType,
  type MenuFormValue,
} from "./MenuFormDialog";
import { MoveMenuDialog } from "./MoveMenuDialog";

type Dialog =
  | { kind: "edit" }
  | { kind: "move" }
  /** existing — пункт заводят на уже существующую таблицу, а не на новую. */
  | { kind: "create"; type: CreatableType; existing?: boolean }
  | { kind: "delete" }
  | { kind: "template" }
  | null;

/**
 * Кнопка «⋮» на строке меню. Набор пунктов собирается из реестра действий
 * и прав самого пункта — ветвлений по типу здесь нет.
 */
export function MenuRowActions({ node }: { node: MenuNode }) {
  const { t } = useTranslation();
  const [dialog, setDialog] = useState<Dialog>(null);
  /*
   * Роль спрашиваем здесь, а не принимаем сверху: «сделать шаблоном»
   * видит только суперадмин, и пока это был проброшенный флаг, строка
   * меню передавала в него голое false — действие не показывалось
   * никому и никогда.
   */
  const isAdmin = useIsSuperRole();

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
    if (id === "move") setDialog({ kind: "move" });
    if (id === "create-folder") setDialog({ kind: "create", type: "FOLDER" });
    if (id === "create-table") setDialog({ kind: "create", type: "TABLE" });
    if (id === "link-table") setDialog({ kind: "create", type: "TABLE", existing: true });
    if (id === "create-link") setDialog({ kind: "create", type: "LINK" });
    if (id === "create-files") setDialog({ kind: "create", type: "MINIO_FOLDER" });
    if (id === "create-microfrontend") setDialog({ kind: "create", type: "MICROFRONTEND" });
    if (id === "delete") setDialog({ kind: "delete" });
    if (id === "make-template") setDialog({ kind: "template" });
  };

  const submitForm = (value: MenuFormValue) => {
    if (dialog?.kind === "edit") {
      update.mutate(
        {
          node,
          labels: value.labels,
          icon: value.icon,
          attributes: menuAttributes(node.type, value),
        },
        { onSuccess: () => setDialog(null) },
      );
    } else if (dialog?.kind === "create") {
      create.mutate(
        {
          labels: value.labels,
          icon: value.icon,
          type: dialog.type,
          parentId: node.id,
          // Слаг — только у НОВОЙ таблицы; у существующей вместо него
          // её идентификатор, и создаётся один пункт меню, без таблицы.
          ...(dialog.type === "TABLE" && !dialog.existing ? { slug: value.slug } : {}),
          ...(dialog.existing ? { tableId: value.tableId } : {}),
          ...(dialog.type === "MICROFRONTEND"
            ? { microfrontendId: value.microfrontendId }
            : {}),
          attributes: menuAttributes(dialog.type, value),
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
              <PopoverItem
                danger={action.danger ?? false}
                icon={
                  <Icon
                    as={action.icon}
                    size={16}
                    className={action.danger ? "shrink-0" : "shrink-0 text-fg-muted"}
                  />
                }
                onClick={() => run(action.id, close)}
              >
                {t(action.labelKey, { type: typeWord })}
              </PopoverItem>
            </div>
          ))
        }
      </Popover>

      {dialog?.kind === "edit" && (
        <MenuFormDialog
          title={t("menuForm.editTitle", { type: typeWord })}
          initial={{
            ...EMPTY_MENU_FORM,
            labels: node.labels,
            icon: node.icon,
            href: node.href ?? "",
            folder: node.folder,
            embed: Boolean(node.embedUrl),
            microfrontendId: node.microfrontendId,
            params: Object.entries(node.params).map(([key, value]) => ({ key, value })),
          }}
          type={node.type}
          busy={update.isPending}
          onSubmit={submitForm}
          onClose={() => setDialog(null)}
        />
      )}

      {dialog?.kind === "create" && (
        <MenuFormDialog
          title={t(dialog.existing ? "menuForm.linkTable" : CREATE_TITLES[dialog.type])}
          initial={EMPTY_MENU_FORM}
          type={dialog.type}
          needsSlug={dialog.type === "TABLE" && !dialog.existing}
          needsTable={Boolean(dialog.existing)}
          needsRemote={dialog.type === "MICROFRONTEND"}
          busy={create.isPending}
          onSubmit={submitForm}
          onClose={() => setDialog(null)}
        />
      )}

      {dialog?.kind === "move" && (
        <MoveMenuDialog node={node} onClose={() => setDialog(null)} />
      )}

      {dialog?.kind === "template" && (
        <TemplateCreateDialog
          menuId={node.id}
          menuLabel={node.label}
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

