import { Link, useRouterState } from "@tanstack/react-router";
import { Bell, Bot, Clock, Moon, Plus, Sun } from "lucide-react";
import { BlippyLogo } from "@/components/blippy-logo";
import { useTheme } from "@/components/theme-provider";
import { Button } from "@/components/ui/button";
import {
	Sidebar,
	SidebarContent,
	SidebarFooter,
	SidebarGroup,
	SidebarGroupContent,
	SidebarGroupLabel,
	SidebarHeader,
	SidebarMenu,
	SidebarMenuButton,
	SidebarMenuItem,
} from "@/components/ui/sidebar";

export function AppSidebar() {
	const { theme, setTheme } = useTheme();
	const routerState = useRouterState();
	const pathname = routerState.location.pathname;

	const isActive = (path: string) => {
		if (path === "/") return pathname === "/";
		return pathname.startsWith(path);
	};

	const toggleTheme = () => {
		setTheme(theme === "dark" ? "light" : theme === "light" ? "dark" : "light");
	};

	return (
		<Sidebar>
			<SidebarHeader>
				<SidebarMenu>
					<SidebarMenuItem>
						<SidebarMenuButton size="lg" asChild>
							<Link to="/" className="!gap-3">
								<BlippyLogo className="!size-8 shrink-0" />
								<span className="text-lg font-semibold">Blippy</span>
							</Link>
						</SidebarMenuButton>
					</SidebarMenuItem>
				</SidebarMenu>
			</SidebarHeader>

			<SidebarContent>
				<SidebarGroup>
					<SidebarGroupLabel>Agents</SidebarGroupLabel>
					<SidebarGroupContent>
						<SidebarMenu>
							<SidebarMenuItem>
								<SidebarMenuButton asChild isActive={isActive("/")}>
									<Link to="/">
										<Bot className="size-4" />
										<span>All Agents</span>
									</Link>
								</SidebarMenuButton>
							</SidebarMenuItem>
							<SidebarMenuItem>
								<SidebarMenuButton asChild>
									<Link to="/agents/new">
										<Plus className="size-4" />
										<span>New Agent</span>
									</Link>
								</SidebarMenuButton>
							</SidebarMenuItem>
						</SidebarMenu>
					</SidebarGroupContent>
				</SidebarGroup>

				<SidebarGroup>
					<SidebarGroupLabel>Automation</SidebarGroupLabel>
					<SidebarGroupContent>
						<SidebarMenu>
							<SidebarMenuItem>
								<SidebarMenuButton asChild isActive={isActive("/triggers")}>
									<Link to="/triggers">
										<Clock className="size-4" />
										<span>Triggers</span>
									</Link>
								</SidebarMenuButton>
							</SidebarMenuItem>
							<SidebarMenuItem>
								<SidebarMenuButton
									asChild
									isActive={isActive("/notifications")}
								>
									<Link to="/notifications">
										<Bell className="size-4" />
										<span>Notifications</span>
									</Link>
								</SidebarMenuButton>
							</SidebarMenuItem>
						</SidebarMenu>
					</SidebarGroupContent>
				</SidebarGroup>
			</SidebarContent>

			<SidebarFooter className="mt-auto pb-[env(safe-area-inset-bottom)] md:pb-2">
				<SidebarMenu>
					<SidebarMenuItem>
						<SidebarMenuButton asChild>
							<Button
								variant="ghost"
								className="w-full justify-start"
								onClick={toggleTheme}
							>
								{theme === "dark" ? (
									<Sun className="size-4" />
								) : (
									<Moon className="size-4" />
								)}
								<span>{theme === "dark" ? "Light Mode" : "Dark Mode"}</span>
							</Button>
						</SidebarMenuButton>
					</SidebarMenuItem>
				</SidebarMenu>
			</SidebarFooter>
		</Sidebar>
	);
}
