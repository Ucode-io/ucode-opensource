import { useTranslation } from "react-i18next";
import { Checkbox } from "@/shared/ui/checkbox";
import { Dropdown } from "@/shared/ui/dropdown";
import { Input } from "@/shared/ui/input";
import {
  hasMapSettings,
  hasPhotoSettings,
  hasScannerSettings,
  hasTranscode,
  PHOTO_RATIOS,
  type FieldDraft,
} from "../model/field-draft";
import { Labeled } from "./FormulaSettings";

/**
 * Настройки, которые есть у одного типа и бессмысленны у остальных:
 * точка на карте, пропорции снимка, перекодирование видео, поведение
 * сканера.
 *
 * Собраны в одном месте по той же причине, по какой в старой админке
 * их собирал `Attributes/index.jsx`: это не общая настройка поля,
 * а частность типа, и в общем списке она сбивала бы с толку у всех
 * остальных.
 *
 * Каждая из них кем-то исполняется: `lat`/`long` открывают пустую
 * ячейку карты на заданной точке, `ratio` уезжает с загрузкой снимка
 * и обрезает его (`file.go:133`), `pressEnter`/`length` говорят полю
 * сканера, когда код дочитан. Спрашивать настройку, которую не читает
 * никто, здесь больше не надо — так ушли «формат снимка» и «ключ карт»
 * (docs/FIELD-AUDIT.md, F21 и F25): наш ключ зашит в `shared/lib/
 * yandex-maps`, а формат бэкенд не читает вовсе и кодирует снимок
 * обратно в том, в котором тот пришёл. Уже записанные значения обоих
 * ключей остаются в attributes нетронутыми: тело правки собирается
 * поверх прежних (api/fields, toUpdateBody).
 */
export function TypeSettings({
  draft,
  onChange,
}: {
  draft: FieldDraft;
  onChange: (next: Partial<FieldDraft>) => void;
}) {
  const { t } = useTranslation();

  if (hasMapSettings(draft.type)) {
    return (
      <div className="flex flex-col gap-1.5 px-2 py-1">
        <div className="flex gap-1.5">
          <Labeled label={t("cell.latitude")}>
            <Input
              value={draft.lat}
              inputMode="decimal"
              placeholder="41.311"
              onChange={(event) => onChange({ lat: event.target.value })}
              className="h-7 px-1.5 text-xs tabular-nums"
            />
          </Labeled>

          <Labeled label={t("cell.longitude")}>
            <Input
              value={draft.long}
              inputMode="decimal"
              placeholder="69.240"
              onChange={(event) => onChange({ long: event.target.value })}
              className="h-7 px-1.5 text-xs tabular-nums"
            />
          </Labeled>
        </div>
      </div>
    );
  }

  if (hasPhotoSettings(draft.type)) {
    return (
      <div className="flex flex-col gap-1.5 px-2 py-1">
        <Labeled label={t("fieldForm.photoRatio")}>
          <Dropdown
            size="sm"
            value={draft.ratio}
            placeholder="—"
            items={[
              ...PHOTO_RATIOS.map((ratio) => ({ value: ratio.value, label: ratio.label })),
              /* Пропорция из старых настроек, которой нет в списке:
                 в attributes лежит число, и произвольное там законно. */
              ...(draft.ratio && !PHOTO_RATIOS.some((ratio) => ratio.value === draft.ratio)
                ? [{ value: draft.ratio, label: draft.ratio }]
                : []),
            ]}
            onChange={(ratio) => onChange({ ratio })}
          />
        </Labeled>
      </div>
    );
  }

  if (hasTranscode(draft.type)) {
    return (
      <label className="flex h-8 cursor-pointer items-center gap-2 px-2">
        <Checkbox
          checked={draft.transcode}
          onChange={(event) => onChange({ transcode: event.target.checked })}
        />
        <span className="flex-1 truncate text-sm text-fg">{t("fieldForm.transcode")}</span>
      </label>
    );
  }

  if (hasScannerSettings(draft.type)) {
    return (
      <>
        <label className="flex h-8 cursor-pointer items-center gap-2 px-2">
          <Checkbox
            checked={draft.pressEnter}
            onChange={(event) => onChange({ pressEnter: event.target.checked })}
          />
          <span className="flex-1 truncate text-sm text-fg">{t("fieldForm.pressEnter")}</span>
        </label>

        <label className="flex h-8 items-center gap-2 px-2">
          <span className="flex-1 truncate text-sm text-fg">{t("fieldForm.codeLength")}</span>
          <input
            value={draft.length}
            inputMode="numeric"
            placeholder="13"
            onChange={(event) => onChange({ length: event.target.value.replace(/\D/g, "") })}
            className="h-6 w-12 rounded-md border border-border-strong bg-surface px-1.5 text-center text-xs tabular-nums text-fg outline-none focus:border-accent"
          />
        </label>
      </>
    );
  }

  return null;
}
