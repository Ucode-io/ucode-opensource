import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { ProductPreview } from "./ProductPreview";

/** Общий каркас экранов входа и регистрации: форма слева, бренд справа. */
export function AuthLayout({ children }: { children: ReactNode }) {
  return (
    <div className="grid min-h-dvh lg:grid-cols-2">
      <div className="grid place-items-center p-6">{children}</div>
      <BrandPanel />
    </div>
  );
}

/**
 * Единственное место в продукте с градиентом. Здесь можно: экран без
 * данных, читать нечего, сканировать нечего.
 *
 * Светлый конец градиента — #0075cf, на нём белый текст даёт 4.72:1.
 * Растянуть градиент до самого бренда #45aeff нельзя: там 2.40:1,
 * и контраст текста стал бы диапазоном вместо значения.
 */
function BrandPanel() {
  const { t } = useTranslation();

  return (
    <div
      aria-hidden
      className="relative hidden overflow-hidden bg-linear-160 from-brand-gradient-from from-20% via-brand-gradient-via via-55% to-brand-gradient-to lg:block"
    >
      {/* Сетка тонких линий — намёк на таблицу, из которой состоит продукт */}
      <div
        className="absolute inset-0 opacity-20"
        style={{
          backgroundImage:
            "linear-gradient(#ffffff26 1px, transparent 1px), linear-gradient(90deg, #ffffff26 1px, transparent 1px)",
          backgroundSize: "36px 36px",
        }}
      />

      <div className="relative flex h-full flex-col justify-center gap-10 p-12">
        <div className="flex flex-col gap-3 text-white">
          <p className="text-2xl font-semibold tracking-tight">{t("auth.tagline")}</p>
          <p className="max-w-sm text-base text-white/70">{t("auth.taglineNote")}</p>
        </div>

        {/* Макет уезжает вправо за край — так читается как окно продукта,
            а не как картинка, вставленная в панель. */}
        <div className="-mr-24">
          <ProductPreview />
        </div>
      </div>
    </div>
  );
}
