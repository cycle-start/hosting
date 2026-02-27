export function ErrorAlert({ message }: { message: string }) {
  return (
    <div className="mb-4 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700">
      {message}
    </div>
  );
}
