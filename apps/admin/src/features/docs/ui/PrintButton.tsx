import { useMemo, useRef, useState } from "react";
import { IconCode, IconFileTypeDocx, IconLoader2, IconPrinter } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import type { Field } from "@/features/table";
import { Anchored } from "@/shared/ui/anchored";
import { Icon } from "@/shared/ui/icon";
import { useHtmlTemplates, useHtmlToPdf } from "../api/html-templates";
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
  fields,
}: {
  tableSlug: string;
  /** Строка целиком: что не прислали, того в документе не будет. */
  row: Record<string, unknown>;
  /**
   * Поля таблицы. Нужны ровно для одного: отличить связь Many2One
   * от обычного поля, в слаге которого есть «_id» — на втором печать
   * падает целиком (см. model/print-data).
   */
  fields: Field[];
}) {
  const { t } = useTranslation();
  const { templates } = useDocTemplates(tableSlug);
  const html = useHtmlTemplates(tableSlug);
  const print = usePrintItem(tableSlug);
  const printHtml = useHtmlToPdf();
  const lookups = useMemo(
    () => new Set(fields.filter((field) => field.type === "LOOKUP").map((field) => field.slug)),
    [fields],
  );
  const button = useRef<HTMLButtonElement>(null);
  const [anchor, setAnchor] = useState<DOMRect | null>(null);

  const busy = print.isPending || printHtml.isPending;

  /*
   * Оба вида печатных форм в одном меню: человек выбирает документ,
   * а не способ его сборки. Что docx собирается ручкой одного
   * поколения, а HTML — другого, его не касается.
   */
  const total = templates.length + html.templates.length;
  if (!total) return null;

  const only = total === 1 ? templates[0] : undefined;
  const onlyHtml = total === 1 ? html.templates[0] : undefined;

  return (
    <>
      <button
        ref={button}
        type="button"
        disabled={busy}
        onClick={() => {
          if (only) return void print.mutate({ template: only, row, lookups });
          if (onlyHtml) return void printHtml.mutate({ template: onlyHtml, row, lookups });
          setAnchor(button.current?.getBoundingClientRect() ?? null);
        }}
        aria-label={t("docs.print")}
        title={t("docs.print")}
        className="grid size-7 place-items-center rounded-md text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg disabled:opacity-40"
      >
        <Icon
          as={busy ? IconLoader2 : IconPrinter}
          size={16}
          className={busy ? "animate-spin" : ""}
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
                  print.mutate({ template, row, lookups });
                  setAnchor(null);
                }}
                className="flex h-8 w-full items-center gap-2 rounded-md px-2 text-left text-sm text-fg transition-colors hover:bg-surface-hover"
              >
                <Icon as={IconFileTypeDocx} size={14} className="shrink-0 text-fg-subtle" />
                <span className="truncate">{template.title || t("docs.untitled")}</span>
              </button>
            ))}

            {html.templates.map((template) => (
              <button
                key={template.id}
                type="button"
                onClick={() => {
                  printHtml.mutate({ template, row, lookups });
                  setAnchor(null);
                }}
                className="flex h-8 w-full items-center gap-2 rounded-md px-2 text-left text-sm text-fg transition-colors hover:bg-surface-hover"
              >
                <Icon as={IconCode} size={14} className="shrink-0 text-fg-subtle" />
                <span className="truncate">{template.title || t("docs.untitled")}</span>
              </button>
            ))}
          </div>
        </Anchored>
      )}
    </>
  );
}
