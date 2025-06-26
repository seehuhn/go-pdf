# Load necessary libraries
library(ggplot2)
library(dplyr)

# Read the CSV file
df <- read.csv("data.csv")

# Create a group variable for (colors, depth)
df <- df %>%
  mutate(group = interaction(colors, depth, sep = "_"))

# Normalize length by the minimum within each group
df <- df %>%
  group_by(group) %>%
  mutate(length_std = length / min(length)) %>%
  ungroup()

# Convert predictor to a factor with specified order
df$predictor <- factor(df$predictor, levels = c(1, 2, 10, 11, 12, 13, 14, 15))

# Plot
ggplot(df, aes(x = predictor, y = length_std, group = group, colour = group)) +
  geom_line() +
  geom_point() +
  labs(
    x = "Predictor",
    y = "Standardized Length (Min = 1 per group)",
    colour = "Color-Depth Group"
  ) +
  theme_minimal()
