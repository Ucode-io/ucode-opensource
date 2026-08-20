import { useRef, useState } from "react";
import { IconFileSpreadsheet, IconLoader2 } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { localized, type Field } from "@/features/table";
import { Button } from "@/shared/ui/button";
import { Icon } from "@/shared/ui/icon";
import { Modal } from "@/shared/ui/modal";
import { useImportExcel, useReadExcel } from "../api/excel";

/**
 * Загрузка строк из Excel.
 *
 * Два шага в одном окне: выбрать файл, потом сопоставить его столбцы
 * с полями таблицы. Второй шаг обязателен и не сворачивается в «мы всё
 * угадали»: в шапке файла человеческие названия, а не слаги, и
 * молчаливая догадка пишет данные не в те колонки — а это уже не
 * отменить кнопкой.
 *
 * Сопоставление идёт от ПОЛЯ к столбцу, а не наоборот: полей у таблицы
 * конечное известное число, а столбцов в чужом файле бывает пятьдесят,
 * из которых нужны три.
 */
export function ExcelImportDialog({
  tableSlug,
  fields,
  language,
  onClose,
}: {
  tableSlug: string;
  /** Поля таблицы: заполнить можно любое, а не только показанное во view. */
  fields: Field[];
  language: string;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const input = useRef<HTMLInputElement>(null);

  const read = useReadExcel();
  const write = useImportExcel(tableSlug);

  const [file, setFile] = useState<{ excelId: string; columns: string[] } | null>(null);
  /** Поле → столбец файла. Нет ключа — поле не заполняется. */
  const [mapping, setMapping] = useState<Record<string, string>>({});

  const pick = (chosen: File | undefined) => {
    if (!chosen) return;

    read.mutate(chosen, {
      onSuccess: (result) => {
        setFile(result);
        /*
         * Предзаполняем только точные совпадения подписи со столбцом.
         * «Похоже» здесь не годится: «Цена» и «Цена закупки» отличаются
         * одним словом и парой тысяч рублей в каждой строке.
         */
        const byLabel = new Map(result.columns.map((column) => [column.trim().toLowerCase(), column]));
        const guessed: Record<string, string> = {};

        for (const field of fields) {
          const column = byLabel.get(localized(field.labels, language, field.label).trim().toLowerCase());
          if (column) guessed[field.id] = column;
        }

        setMapping(guessed);
      },
    });
  };

  const submit = () => {
    /* Карта переворачивается на отправку: ручка ждёт «столбец → поле». */
    const body: Record<string, string> = {};
    for (const [fieldId, column] of Object.entries(mapping)) {
      if (column) body[column] = fieldId;
    }

    write.mutate({ excelId: file?.excelId ?? "", mapping: body }, { onSuccess: onClose });
  };

  const chosen = Object.values(mapping).filter(Boolean).length;

  return (
    <Modal onClose={onClose}>
      <div className="flex max-h-[80vh] w-full max-w-lg flex-col gap-3 rounded-xl border border-border bg-surface p-5 shadow-modal">
        <h2 className="text-base font-semibold">{t("view.import")}</h2>

        {!file ? (
          <>
            <p className="text-sm text-fg-muted">{t("view.importHint")}</p>

            <input
              ref={input}
              type="file"
              accept=".xlsx,.xls"
              className="hidden"
              onChange={(event) => pick(event.target.files?.[0])}
            />

            <button
              type="button"
              onClick={() => input.current?.click()}
              disabled={read.isPending}
              className="flex flex-col items-center gap-2 rounded-lg border border-dashed border-border-strong p-8 text-sm text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg disabled:pointer-events-none disabled:opacity-50"
            >
              <Icon as={read.isPending ? IconLoader2 : IconFileSpreadsheet} size={24} className={read.isPending ? "animate-spin" : ""} />
              {t(read.isPending ? "common.loading" : "view.importPick")}
            </button>
          </>
        ) : (
          <>
            <p className="text-sm text-fg-muted">{t("view.importMapHint")}</p>

            <div className="-mx-1 min-h-0 flex-1 overflow-y-auto px-1">
              {fields.map((field) => (
                <label key={field.id} className="flex h-9 items-center gap-2 text-sm">
                  <span className="min-w-0 flex-1 truncate text-fg-muted">
                    {localized(field.labels, language, field.label)}
                  </span>

                  <select
                    value={mapping[field.id] ?? ""}
                    onChange={(event) =>
                      setMapping((prev) => ({ ...prev, [field.id]: event.target.value }))
                    }
                    className="h-8 w-56 shrink-0 rounded-md border border-border-strong bg-surface px-2 text-sm text-fg"
                  >
                    <option value="">{t("view.importSkip")}</option>
                    {file.columns.map((column) => (
                      <option key={column} value={column}>
                        {column}
                      </option>
                    ))}
                  </select>
                </label>
              ))}
            </div>
          </>
        )}

        <div className="mt-1 flex items-center justify-end gap-2">
          {file && <span className="mr-auto text-xs text-fg-subtle">{t("view.importChosen", { count: chosen })}</span>}

          <Button variant="ghost" onClick={onClose}>
            {t("action.cancel")}
          </Button>

          {file && (
            <Button disabled={!chosen || write.isPending} onClick={submit}>
              {t("view.import")}
            </Button>
          )}
        </div>
      </div>
    </Modal>
  );
}
