import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { Modal } from "@/shared/ui/modal";
import { SelectMenu } from "@/shared/ui/select-menu";
import { ROOT_MENU_ID, useMenuTree } from "../api/menus";
import { useUpdateMenu } from "../api/mutations";
import { moveTargets } from "../model/reorder";
import { matchMenus } from "../model/search";
import type { MenuNode } from "../model/types";

/**
 * Перенос пункта в другую папку.
 *
 * Старое «Move …» рисовало дерево с раскрывающимися ветками
 * (FolderModalComponent.jsx). Здесь тот же выбор одним списком: дорога
 * до папки написана в самой строке, и до вложенной папки не нужно
 * раскрывать три уровня.
 *
 * Дерево читается тем же обходом, что и поиск по сайдбару (useMenuTree),
 * и отбор набранного — тем же matchMenus: список папок и список
 * найденного — это один и тот же список.
 */
export function MoveMenuDialog({ node, onClose }: { node: MenuNode; onClose: () => void }) {
  const { t } = useTranslation();
  const [search, setSearch] = useState("");
  // Изначально выбрана текущая папка: окно открывают, чтобы её сменить,
  // и видеть, откуда переносят, — половина ответа на вопрос «куда».
  const [parentId, setParentId] = useState(node.parentId ?? ROOT_MENU_ID);
  const tree = useMenuTree(true);
  const update = useUpdateMenu();

  const targets = moveTargets(tree.items, node);
  const found = search.trim() ? matchMenus(targets, search) : targets;

  // Корень — первой строкой и всегда: пункт выносят на верхний уровень
  // так же часто, как убирают в папку, а строки в дереве у корня нет.
  const items = [
    { value: ROOT_MENU_ID, label: t("menuForm.moveRoot") },
    ...found.map((match) => ({
      value: match.node.id,
      label: [...match.trail.map((step) => step.label), match.node.label].join(" / "),
      icon: match.node.icon,
    })),
  ];

  return (
    <Modal onClose={onClose}>
      <div className="flex w-full max-w-sm flex-col gap-4 rounded-xl border border-border bg-surface p-5 shadow-modal">
        <h2 className="text-base font-semibold">
          {t("menuForm.moveTitle", { label: node.label })}
        </h2>

        <SelectMenu
          label={t("menuForm.moveTo")}
          placeholder={t("menuForm.movePick")}
          searchPlaceholder={t("menuForm.moveSearch")}
          emptyText={tree.isLoading ? t("common.loading") : t("menuForm.moveEmpty")}
          items={items}
          selected={new Set([parentId])}
          search={search}
          loading={tree.isLoading}
          onSearch={setSearch}
          onLoadMore={() => {}}
          onPick={setParentId}
        />

        <div className="flex justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button
            type="button"
            // Та же папка — переносить нечего, а PUT перезаписал бы строку
            // целиком ради ничего.
            disabled={update.isPending || parentId === node.parentId}
            onClick={() => update.mutate({ node, parentId }, { onSuccess: onClose })}
          >
            {t("action.save")}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
