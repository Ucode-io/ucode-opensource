import { Tabs } from "@/shared/ui/tabs";

/**
 * Переключатель языка ДАННЫХ: коды в ряд, выбранный подсвечен.
 *
 * Та же полоса, что и остальные вкладки приложения (shared/ui/tabs):
 * свой вид у него был акцентной заливкой — как у активной вкладки связи
 * до того, как их свели, — и три чужеродных чипа висели в углу шапки.
 *
 * Кодами, а не полными именами: языков у проекта бывает четыре, а места
 * рядом с кнопками записи — на два слова. Полное имя показывается
 * подсказкой.
 *
 * Ничего не решает сам: язык данных один на приложение, поэтому значение
 * и обработчик приходят снаружи.
 */
export function LanguageTabs({
  languages,
  value,
  onChange,
}: {
  languages: { code: string; nativeName: string }[];
  value: string;
  onChange: (code: string) => void;
}) {
  // Одному языку переключаться некуда.
  if (languages.length < 2) return null;

  return (
    <Tabs
      tabs={languages.map((language) => ({
        id: language.code,
        label: language.code,
        title: language.nativeName,
      }))}
      activeId={value}
      onSelect={onChange}
    />
  );
}
