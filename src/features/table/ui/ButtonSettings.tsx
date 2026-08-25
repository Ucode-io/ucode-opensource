import { useTranslation } from "react-i18next";
import { IconPicker } from "@/features/icons";
import { Select } from "@/shared/ui/input";
import { useFunctions } from "../api/functions";
import type { FieldDraft } from "../model/field-draft";
import { Labeled } from "./FormulaSettings";

/**
 * Функция поля и (у кнопки) её иконка.
 *
 * Полей с функцией два: BUTTON зовёт её кликом, SCAN_BARCODE — когда
 * сканер дочитал код. Иконка есть только у первого: у сканера в ячейке
 * рисунок кода, а не значок.
 *
 * У кнопки значения нет вовсе — колонка в базе пустая всегда, — поэтому
 * здесь нет ни умолчания, ни проверки ввода: настраивать нечего, кроме
 * действия.
 *
 * Компонент монтируется только у этих двух типов: список функций —
 * отдельный запрос, и у остальных полей он уходил бы в никуда на каждое
 * открытие панели.
 */
export function ButtonSettings({
  draft,
  icon = true,
  onChange,
}: {
  draft: FieldDraft;
  /** Показывать выбор иконки. У сканера её нет. */
  icon?: boolean;
  onChange: (next: Partial<FieldDraft>) => void;
}) {
  const { t } = useTranslation();
  const { functions, isLoading } = useFunctions();

  return (
    <div className="flex flex-col gap-1.5 px-2 py-1">
      {icon && (
        <Labeled label={t("button.icon")}>
          {/* Тот же пикер, что у пунктов меню: формат имени общий —
              «tabler:bolt», ссылка или файл из нашего CDN. */}
          <IconPicker
            value={draft.icon}
            type={draft.type}
            onChange={(next) => onChange({ icon: next })}
          />
        </Labeled>
      )}

      <Labeled label={t("button.function")}>
        <Select
          value={draft.functionId}
          onChange={(event) => onChange({ functionId: event.target.value })}
          className="h-7 px-1.5 text-xs"
        >
          <option value="">—</option>
          {/*
            Выбранная функция могла быть удалена — тогда её нет в списке,
            и select показал бы первую попавшуюся. Держим её отдельной
            строкой: настройка видна, и случайной подмены не происходит.
          */}
          {draft.functionId && !functions.some((item) => item.id === draft.functionId) && (
            <option value={draft.functionId}>{draft.functionId}</option>
          )}
          {functions.map((item) => (
            <option key={item.id} value={item.id}>
              {item.name}
            </option>
          ))}
        </Select>
      </Labeled>

      {/* Без функции кнопка нарисуется, но ничего не сделает: сказать
          об этом надо в настройках, а не молчать до первого клика. */}
      {!isLoading && !functions.length && (
        <p className="text-2xs text-fg-subtle">{t("button.noFunctions")}</p>
      )}
    </div>
  );
}
