export default function AuthLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-full items-center justify-center bg-[var(--background)] p-6">
      <div className="w-full max-w-md">{children}</div>
    </div>
  );
}
