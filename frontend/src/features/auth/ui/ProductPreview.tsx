import { Chip } from "@/shared/ui/chip";

/**
 * Окно продукта на панели входа. Не скриншот и не картинка — та же
 * вёрстка на тех же токенах, что и настоящий экран. Значит, оно
 * не устареет молча: поменяются токены — поменяется и превью.
 *
 * Сайдбар нарисован вместе с таблицей не для красоты: без него это
 * карточка с данными, а с ним — окно приложения, и понятно, что
 * за продуктом стоит целый экран, а не одна таблица.
 *
 * Данные английские и выдуманные. Внутри aria-hidden панели, поэтому
 * скринридер их не читает.
 */

/*
 * Значки нарисованы здесь, а не взяты из набора: это единственное место,
 * где они нужны, и тянуть ради шести штук лишние импорты в бандл входа
 * незачем. Все — 14×14 на одной сетке.
 */
function icon(path: string) {
  return function Icon() {
    return (
      <svg
        width="14"
        height="14"
        viewBox="0 0 16 16"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.3"
        strokeLinecap="round"
        strokeLinejoin="round"
        className="shrink-0"
      >
        <path d={path} />
      </svg>
    );
  };
}

const HomeIcon = icon("M2.5 7 8 2.5 13.5 7v6a1 1 0 0 1-1 1h-9a1 1 0 0 1-1-1V7Z");
const CartIcon = icon("M2 2.5h1.5l1.5 8h7l1.5-5.5H5M6 13.5h.01M11 13.5h.01");
const BoxIcon = icon("M8 2 13.5 5v6L8 14 2.5 11V5L8 2Zm0 0v12M2.5 5 8 8l5.5-3");
const UserIcon = icon("M8 8a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5ZM3 14c0-2.2 2.2-3.5 5-3.5s5 1.3 5 3.5");
const BankIcon = icon("M2.5 6.5 8 3l5.5 3.5M4 7v5m4-5v5m4-5v5M2.5 13.5h11");
const ChartIcon = icon("M3 13.5V7m5 6.5V3.5m5 10V9");
const StoreIcon = icon("M2.5 6.5h11l-1 7h-9l-1-7Zm2.5 0V4a3 3 0 0 1 6 0v2.5");
const PlusIcon = icon("M8 3.5v9M3.5 8h9");

const MENU = [
  { label: "Home", icon: HomeIcon },
  { label: "Orders", icon: CartIcon, badge: "15" },
  { label: "Products", icon: BoxIcon },
  { label: "Customers", icon: UserIcon, active: true },
  { label: "Finances", icon: BankIcon },
  { label: "Analytics", icon: ChartIcon },
] as const;

const ROWS = [
  { name: "Brandon Clark", company: "airbnb.com", status: "In progress", color: "blue", value: "24,000" },
  { name: "Mia Rodriguez", company: "figma.com", status: "Done", color: "green", value: "8,300" },
  { name: "Ryan Mitchell", company: "dropbox.com", status: "On review", color: "yellow", value: "12,500" },
  { name: "Sarah Reynolds", company: "notion.so", status: "Not started", color: "gray", value: "4,200" },
  { name: "David Larson", company: "stripe.com", status: "In progress", color: "blue", value: "31,000" },
  { name: "Emma Thompson", company: "slack.com", status: "Done", color: "green", value: "16,750" },
  { name: "Alex Chen", company: "vercel.com", status: "On review", color: "yellow", value: "9,900" },
  { name: "Olivia Bennett", company: "linear.app", status: "Done", color: "green", value: "21,400" },
  { name: "Marcus Hall", company: "shopify.com", status: "In progress", color: "blue", value: "18,200" },
  { name: "Priya Nair", company: "airtable.com", status: "Not started", color: "gray", value: "6,050" },
  { name: "Tom Weber", company: "sentry.io", status: "Done", color: "green", value: "27,800" },
] as const;

export function ProductPreview() {
  return (
    /*
     * Высота — во всю отведённую площадь, и нижний край уходит под край
     * панели: окно продолжается за экраном, а не заканчивается ровно
     * там, где кончились строки.
     */
    <div className="flex h-full overflow-hidden rounded-2xl border border-border bg-surface shadow-modal">
      <Sidebar />

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex shrink-0 items-center justify-between gap-3 px-4 py-3">
          <span className="text-base font-semibold text-fg">Customers</span>
          <div className="flex items-center gap-1.5">
            <span className="rounded-md border border-border px-2 py-1 text-2xs text-fg-muted">
              Last 7 days
            </span>
            <span className="rounded-md border border-border px-2 py-1 text-2xs text-fg-muted">
              Filters
            </span>
          </div>
        </header>

        <div className="shrink-0 border-y border-border px-4 py-2 text-xs text-fg-subtle">
          Search customers
        </div>

        <table className="w-full border-collapse text-left">
          <thead>
            <tr className="text-xs text-fg-muted">
              <th className="h-8 px-4 font-normal">Customer</th>
              <th className="h-8 px-3 font-normal">Company</th>
              <th className="h-8 px-3 font-normal">Status</th>
              <th className="h-8 px-3 text-right font-normal">Deal size</th>
            </tr>
          </thead>
          <tbody>
            {ROWS.map((row) => (
              <tr key={row.name} className="border-t border-border">
                <td className="h-10 px-4">
                  <span className="flex items-center gap-2">
                    <span className="grid size-5 shrink-0 place-items-center rounded-full bg-accent-subtle text-2xs font-semibold text-accent-text">
                      {row.name[0]}
                    </span>
                    <span className="text-sm whitespace-nowrap text-fg">{row.name}</span>
                  </span>
                </td>
                <td className="h-10 px-3">
                  <span className="rounded-sm bg-surface-active px-1.5 py-0.5 text-xs whitespace-nowrap text-fg-muted">
                    {row.company}
                  </span>
                </td>
                <td className="h-10 px-3">
                  <Chip color={row.color}>{row.status}</Chip>
                </td>
                <td className="h-10 px-3 text-right text-sm whitespace-nowrap tabular-nums text-fg">
                  ${row.value}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

/** Левая колонка окна: рабочее пространство, кнопка и пункты меню. */
function Sidebar() {
  return (
    <aside className="flex w-52 shrink-0 flex-col gap-3 border-r border-border bg-surface-subtle p-3">
      <div className="flex items-center gap-2">
        <span className="grid size-7 shrink-0 place-items-center rounded-md bg-accent-solid text-xs font-semibold text-accent-fg">
          U
        </span>
        <span className="flex min-w-0 flex-col">
          <span className="truncate text-xs font-medium text-fg">Craftwork</span>
          <span className="truncate text-2xs text-fg-subtle">20 employees</span>
        </span>
      </div>

      <div className="flex h-7 items-center justify-center gap-1.5 rounded-md border border-border bg-surface text-xs text-fg-muted">
        <PlusIcon />
        Add new
      </div>

      <nav className="flex flex-col gap-0.5">
        {MENU.map((item) => (
          <span
            key={item.label}
            className={`flex h-7 items-center gap-2 rounded-md px-2 text-xs ${
              "active" in item && item.active
                ? "bg-surface-active font-medium text-fg"
                : "text-fg-muted"
            }`}
          >
            <item.icon />
            <span className="flex-1 truncate">{item.label}</span>
            {"badge" in item && item.badge && (
              <span className="rounded-sm bg-danger-subtle px-1 text-2xs text-danger">
                {item.badge}
              </span>
            )}
          </span>
        ))}
      </nav>

      <span className="mt-1 px-2 text-2xs text-fg-subtle">Sales channels</span>

      <nav className="flex flex-col gap-0.5">
        <span className="flex h-7 items-center gap-2 rounded-md px-2 text-xs text-fg-muted">
          <StoreIcon />
          Online Store
        </span>
        <span className="flex h-7 items-center gap-2 rounded-md px-2 text-xs text-fg-muted">
          <BoxIcon />
          Store Extensions
        </span>
      </nav>
    </aside>
  );
}
