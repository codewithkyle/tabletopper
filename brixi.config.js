module.exports = {
    outDir: "./brixi",
    important: true,
    output: "production",
    baseUnit: "rem",
    features: {
        aspectRatios: true,
        borders: true,
        containers: true,
        cursors: true,
        flexbox: true,
        fonts: true,
        grid: true,
        lineHeight: true,
        margin: true,
        padding: true,
        scroll: true,
        shadows: true,
        positions: true,
        backgrounds: true,
        alignment: true,
        whitespace: true,
        textTransforms: true,
        display: true,
    },
    fonts: {
        units: "rem",
        families: {
            "sans-serif": "Rubik, Ubuntu, 'Open Sans', 'Noto Sans', Roboto, system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Cantarell, 'Helvetica Neue', sans-serif",
            serif: "Georgia, Cambria, 'Times New Roman', Times, serif",
            mono: "Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace",
        },
        weights: {
            hairline: 100,
            thin: 200,
            light: 300,
            regular: 400,
            medium: 500,
            semibold: 600,
            bold: 700,
            heavy: 800,
            black: 900,
        },
        sizes: {
            xs: 0.75,
            sm: 0.875,
            base: 1,
            md: 1.125,
            lg: 1.25,
            xl: 1.5,
            "2xl": 2,
            "3xl": 3,
            "4xl": 4,
        },
    },
    colors: {
        white: "#ffffff",
        black: "#000000",
        grey: {
            50: "#FAFAFA",
            100: "#F4F4F5",
            200: "#E4E4E7",
            300: "#D4D4D8",
            400: "#A1A1AA",
            500: "#71717A",
            600: "#52525B",
            700: "#3F3F46",
            800: "#27272A",
            900: "#18181B",
        },
        primary: {
            50: "#ECFEFF",
            100: "#CFFAFE",
            200: "#A5F3FC",
            300: "#67E8F9",
            400: "#22D3EE",
            500: "#06B6D4",
            600: "#0891B2",
            700: "#0E7490",
            800: "#155E75",
            900: "#164E63",
        },
        success: {
            50: "#ECFDF5",
            100: "#D1FAE5",
            200: "#A7F3D0",
            300: "#6EE7B7",
            400: "#34D399",
            500: "#10B981",
            600: "#059669",
            700: "#047857",
            800: "#065F46",
            900: "#064E3B",
        },
        danger: {
            50: "#FFF1F2",
            100: "#FFE4E6",
            200: "#FECDD3",
            300: "#FDA4AF",
            400: "#FB7185",
            500: "#F43F5E",
            600: "#E11D48",
            700: "#BE123C",
            800: "#9F1239",
            900: "#881337",
        },
        warning: {
            50: "#FFFBEB",
            100: "#FEF3C7",
            200: "#FDE68A",
            300: "#FCD34D",
            400: "#FBBF24",
            500: "#F59E0B",
            600: "#D97706",
            700: "#B45309",
            800: "#92400E",
            900: "#78350F",
        },
        info: {
            50: "#EFF6FF",
            100: "#DBEAFE",
            200: "#BFDBFE",
            300: "#93C5FD",
            400: "#60A5FA",
            500: "#3B82F6",
            600: "#2563EB",
            700: "#1D4ED8",
            800: "#1E40AF",
            900: "#1E3A8A",
        },
    },
    margins: [0, 0.125, 0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.25, 2.5, 2.75, 3, 4, 5, 6],
    padding: [0, 0.125, 0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.25, 2.5, 2.75, 3, 4, 5, 6],
    positions: [0],
    borders: {
        units: "px",
        styles: ["solid", "dashed"],
        widths: [1],
        radius: [0.125, 0.25, 0.5, 1],
    },
    shadows: {
        colors: {
            grey: "0deg 0% 50%",
        },
        sizes: {
            sm: `
                0px 1px 2px hsl(var(--shadow-color) / 0.7)
            `,
            md: `
                0px 2px 2px hsl(var(--shadow-color) / 0.333),
                0px 4px 4px hsl(var(--shadow-color) / 0.333),
                0px 6px 6px hsl(var(--shadow-color) / 0.333)
            `,
            lg: `
                0px 2px 2px hsl(var(--shadow-color) / 0.2),
                0px 4px 4px hsl(var(--shadow-color) / 0.2),
                0px 8px 8px hsl(var(--shadow-color) / 0.2),
                0px 16px 16px hsl(var(--shadow-color) / 0.2),
                0px 32px 32px hsl(var(--shadow-color) / 0.2)
            `,
            xl: `
                0px 2px 2px hsl(var(--shadow-color) / 0.2),
                0px 4px 4px hsl(var(--shadow-color) / 0.2),
                0px 8px 8px hsl(var(--shadow-color) / 0.2),
                0px 16px 16px hsl(var(--shadow-color) / 0.2),
                0px 32px 32px hsl(var(--shadow-color) / 0.2),
                0px 48px 48px hsl(var(--shadow-color) / 0.2),
                0px 64px 64px hsl(var(--shadow-color) / 0.2)
            `,
        },
    },
    containers: {
        units: "px",
        screens: {
            411: 411,
            768: 768,
            1024: 1024,
            1280: 1280,
            1920: 1920,
            3840: 3840,
        },
        columns: [2, 3, 4],
    },
    gaps: [1, 1.5, 2],
    easings: {
        "in-out": "0.4, 0.0, 0.2, 1",
        in: "0.0, 0.0, 0.2, 1",
        out: "0.4, 0.0, 1, 1",
        bounce: "0.175, 0.885, 0.32, 1.275",
    },
    aspectRatios: ["16:9", "4:3", "1:1"],
    variables: {
        "focus-ring": "inset 0 0 0 2px var(--white), 0 0 0 1px var(--black)",
        bevel: "inset 0 -1px 1px hsl(0deg 0% 50% / 0.5), 0 0px 1px hsl(0deg 0% 50% / 0.25), 0 1px 1px hsl(0deg 0% 50% / 0.25)",
        "light-bevel": "0 1px 0 hsl(0deg 0% 50% / 0.1), inset 0 -1px 1px hsl(0deg 0% 50% / 0.15)",
        "primary-opaque-hover": "rgba(14,165,233,0.05)",
        "primary-opaque-active": "rgba(14,165,233,0.1)",
    },
    themes: {},
    classes: {},
};