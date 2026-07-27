"use client";

import { useEffect, useState } from "react";
import { Header } from "@/components/layout/header";
import { Sidebar } from "@/components/layout/sidebar";
import { HeaderSearchProvider } from "@/components/providers/header-search-provider";
import { cn } from "@/lib/utils";

export function AppShell({ children }: { children: React.ReactNode }) {
  const [sidebarOpen, setSidebarOpen] = useState(false);

  useEffect(() => {
    const media = window.matchMedia("(min-width: 1024px)");
    const syncSidebar = (matches: boolean) => setSidebarOpen(matches);
    syncSidebar(media.matches);

    const onChange = (event: MediaQueryListEvent) => syncSidebar(event.matches);
    media.addEventListener("change", onChange);
    return () => media.removeEventListener("change", onChange);
  }, []);

  const toggleSidebar = () => setSidebarOpen((prev) => !prev);
  const closeSidebar = () => setSidebarOpen(false);

  return (
    <HeaderSearchProvider>
      <div className="flex h-screen flex-col overflow-hidden bg-[var(--background)]">
        <div className="shrink-0 overflow-x-auto">
          <Header sidebarOpen={sidebarOpen} onSidebarToggle={toggleSidebar} />
        </div>

        <div className="flex min-h-0 flex-1 overflow-x-auto overflow-y-hidden">
          <div
            className={cn(
              "flex min-w-0 flex-1",
              sidebarOpen ? "lg:min-w-[var(--shell-min-width)]" : "lg:min-w-[var(--dashboard-content-min)]",
            )}
          >
            <Sidebar open={sidebarOpen} onClose={closeSidebar} />

            <div className="flex min-w-[var(--dashboard-content-min)] flex-1 flex-col overflow-hidden">
              {children}
            </div>
          </div>
        </div>
      </div>
    </HeaderSearchProvider>
  );
}
