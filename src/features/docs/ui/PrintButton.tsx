import { useRef, useState } from "react";
import { IconFileTypeDocx, IconLoader2, IconPrinter } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { Anchored } from "@/shared/ui/anchored";
import { Icon } from "@/shared/ui/icon";
import { useDocTemplates, usePrintItem } from "../api/templates";

/**
 * «Напечатать запись» — рядом с самой записью, а не на отдельном экране.
 *
 * Шаблонов у таблицы нет — кнопки нет: пустое меню сообщает только
 * о том, что кто-то другой чего-то не настроил.
 *
 * Один шаблон — печатаем сразу, без выбора из одного пункта. Несколько
 * — открываем список: договор и накладную по одной строке печатают
 * по-разному.
 */
export function PrintButton({
  tableSlug,
  row,
}: {
  tableSlug: string;
  /** Строка целиком: что не прислали, того в документе не будет. */
  row: Record<string, unknown>;
}) {
  const { t } = useTranslation();
  const { templates } = useDocTemplates(tableSlug);
  const print = usePrintItem(tableSlug);
  const button = useRef<HTMLButtonElement>(null);
  const [anchor, setAnchor] = useState<DOMRect | null>(null);

  if (!templates.length) return null;

  const only = templates.length === 1 ? templates[0] : undefined;

  return (
    <>
      <button
        ref={button}
        type="button"
        disabled={print.isPending}
        onClick={() => {
          if (only) return void print.mutate({ template: only, row });
          setAnchor(button.current?.getBoundingClientRect() ?? null);
        }}
        aria-label={t("docs.print")}
        title={t("docs.print")}
        className="grid size-7 place-items-center rounded-md text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg disabled:opacity-40"
      >
        <Icon
          as={print.isPending ? IconLoader2 : IconPrinter}
          size={16}
          className={print.isPending ? "animate-spin" : ""}
        />
      </button>

      {anchor && (
        <Anchored anchor={anchor} onClose={() => setAnchor(null)}>
          <div className="w-56 overflow-hidden rounded-lg border border-border bg-surface p-1 shadow-popover">
            {templates.map((template) => (
              <button
                key={template.id}
                type="button"
                onClick={() => {
                  print.mutate({ template, row });
                  setAnchor(null);
                }}
                className="flex h-8 w-full items-center gap-2 rounded-md px-2 text-left text-sm text-fg transition-colors hover:bg-surface-hover"
              >
                <Icon as={IconFileTypeDocx} size={14} className="shrink-0 text-fg-subtle" />
                <span className="truncate">{template.title || t("docs.untitled")}</span>
              </button>
            ))}
          </div>
        </Anchored>
      )}
    </>
  );
}
