import { type RouteConfig, index, route } from "@react-router/dev/routes";

export default [
  index("routes/home.tsx"),
  route("login", "routes/login.tsx"),
  route("dashboard", "routes/dashboard.tsx"),
  route("catalog", "routes/catalog.tsx"),
  route("organization", "routes/organization.tsx"),
  route("redundancy", "routes/redundancy.tsx"),
  route("dev/kitchen-sink-redundancy", "routes/dev.kitchen-sink-redundancy.tsx"),
] satisfies RouteConfig;
