import { Link } from "react-router";

interface BreadcrumbItem {
  label: string;
  href?: string;
}

export function Breadcrumbs({ items }: { items: BreadcrumbItem[] }) {
  const visible = items.filter((item) => item.label);
  if (visible.length === 0) return null;

  return (
    <nav className="mb-6 text-sm text-gray-500">
      {visible.map((item, i) => (
        <span key={i}>
          {i > 0 && <span className="mx-1.5">/</span>}
          {item.href ? (
            <Link to={item.href} className="hover:text-gray-700">
              {item.label}
            </Link>
          ) : (
            <span className="text-gray-900">{item.label}</span>
          )}
        </span>
      ))}
    </nav>
  );
}
