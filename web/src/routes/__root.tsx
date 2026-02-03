import { createRootRoute, Outlet } from "@tanstack/react-router";
import { Layout } from "@/components/layout";
import { ThemeProvider } from "@/components/theme-provider";
import { Toaster } from "@/components/ui/sonner";

export const Route = createRootRoute({
	component: () => (
		<ThemeProvider defaultTheme="system">
			<Layout>
				<Outlet />
			</Layout>
			<Toaster />
		</ThemeProvider>
	),
});
