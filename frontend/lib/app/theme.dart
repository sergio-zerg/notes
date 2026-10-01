import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

/// The design tokens every screen reads.
///
/// Spacing, radii and durations live here rather than as literals at each call
/// site, so the interface stays visually consistent and a change of rhythm is a
/// one-line edit instead of a sweep through the widgets. The scale is a 4pt
/// grid: every value is a multiple, which is what keeps unrelated rows aligned.
abstract final class AppTokens {
  // Spacing
  static const double xs = 4;
  static const double sm = 8;
  static const double md = 12;
  static const double lg = 16;
  static const double xl = 24;
  static const double xxl = 32;
  static const double xxxl = 48;

  // Corner radii
  static const double radiusSm = 8;
  static const double radiusMd = 12;
  static const double radiusLg = 16;
  static const double radiusXl = 24;
  static const double radiusFull = 999;

  // Interaction
  static const Duration fast = Duration(milliseconds: 150);
  static const Duration medium = Duration(milliseconds: 250);

  /// The width a form stops growing at.
  ///
  /// A text field stretched across a desktop window is unreadable: the caret
  /// ends up nowhere near the label. Forms are centred within this instead.
  static const double maxFormWidth = 440;
}

/// The app theme.
///
/// Both brightnesses are built from one seed colour, so light and dark are two
/// ends of the same palette rather than two designs that drift apart.
abstract final class AppTheme {
  /// The brand seed. A desaturated indigo reads as calm rather than corporate,
  /// and every role derived from it keeps usable contrast in both modes.
  static const Color _seed = Color(0xFF5B6ABF);

  /// The gradient behind the login screen.
  ///
  /// Two stops from the palette itself, so the decoration cannot introduce a
  /// colour the rest of the app does not already use.
  static const List<Color> _brandGradient = [
    Color(0xFF5B6ABF),
    Color(0xFF8E7CC3),
  ];

  static ThemeData light() => _build(Brightness.light);
  static ThemeData dark() => _build(Brightness.dark);

  /// Exposed so a screen that paints its own background uses the same values.
  static List<Color> get brandGradient => _brandGradient;

  /// Whether [theme] is the light variant.
  ///
  /// Next to the theme that produces it, because the question is asked wherever a
  /// surface has to pick a treatment — a shadow or a border — and reading
  /// `theme.brightness` at each site invites a subtly different answer.
  static bool isLight(ThemeData theme) => theme.brightness == Brightness.light;

  static ThemeData _build(Brightness brightness) {
    final isLight = brightness == Brightness.light;
    final scheme = ColorScheme.fromSeed(
      seedColor: _seed,
      brightness: brightness,
    );

    final base = ThemeData(
      colorScheme: scheme,
      brightness: brightness,
      useMaterial3: true,
      // followSystemTheme keeps the app matching the device, which is what a
      // user expects and what the platform guidelines ask for.
      visualDensity: VisualDensity.adaptivePlatformDensity,
    );

    return base.copyWith(
      scaffoldBackgroundColor: isLight
          ? const Color(0xFFF7F8FC)
          : scheme.surface,
      textTheme: _textTheme(base.textTheme, scheme),

      appBarTheme: AppBarTheme(
        backgroundColor: Colors.transparent,
        surfaceTintColor: Colors.transparent,
        foregroundColor: scheme.onSurface,
        elevation: 0,
        scrolledUnderElevation: 0,
        centerTitle: false,
        titleTextStyle: base.textTheme.titleLarge?.copyWith(
          fontWeight: FontWeight.w600,
          color: scheme.onSurface,
          letterSpacing: -0.2,
        ),
        systemOverlayStyle: isLight
            ? SystemUiOverlayStyle.dark
            : SystemUiOverlayStyle.light,
      ),

      cardTheme: CardThemeData(
        elevation: 0,
        color: isLight ? Colors.white : scheme.surfaceContainerHigh,
        // A hairline outline rather than a shadow: shadows read as heavy at
        // rest and blur differently on every platform, while a 1px border keeps
        // the card edge identical on web, iOS and Android.
        surfaceTintColor: Colors.transparent,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppTokens.radiusLg),
          side: BorderSide(
            color: scheme.outlineVariant.withValues(alpha: isLight ? 0.7 : 0.4),
          ),
        ),
        margin: EdgeInsets.zero,
      ),

      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        // A filled field with no border reads lighter than an outlined one and
        // keeps the form from looking like a stack of boxes.
        fillColor: isLight ? Colors.white : scheme.surfaceContainerHigh,
        contentPadding: const EdgeInsets.symmetric(
          horizontal: AppTokens.lg,
          vertical: AppTokens.lg,
        ),
        border: _inputBorder(Colors.transparent),
        enabledBorder: _inputBorder(
          scheme.outlineVariant.withValues(alpha: isLight ? 0.8 : 0.5),
        ),
        focusedBorder: _inputBorder(scheme.primary, width: 1.6),
        errorBorder: _inputBorder(scheme.error),
        focusedErrorBorder: _inputBorder(scheme.error, width: 1.6),
        labelStyle: TextStyle(color: scheme.onSurfaceVariant),
        floatingLabelStyle: TextStyle(
          color: scheme.primary,
          fontWeight: FontWeight.w500,
        ),
        prefixIconColor: scheme.onSurfaceVariant,
        suffixIconColor: scheme.onSurfaceVariant,
      ),

      filledButtonTheme: FilledButtonThemeData(
        style: FilledButton.styleFrom(
          minimumSize: const Size.fromHeight(52),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(AppTokens.radiusMd),
          ),
          textStyle: const TextStyle(
            fontSize: 16,
            fontWeight: FontWeight.w600,
            letterSpacing: 0.1,
          ),
        ),
      ),

      textButtonTheme: TextButtonThemeData(
        style: TextButton.styleFrom(
          minimumSize: const Size(0, 44),
          // 44 is the smallest comfortably tappable target, so the button does
          // not become hard to hit on a phone.
          padding: const EdgeInsets.symmetric(horizontal: AppTokens.md),
          textStyle: const TextStyle(fontSize: 15, fontWeight: FontWeight.w600),
        ),
      ),

      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          minimumSize: const Size.fromHeight(52),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(AppTokens.radiusMd),
          ),
          side: BorderSide(color: scheme.outline),
          textStyle: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
        ),
      ),

      floatingActionButtonTheme: FloatingActionButtonThemeData(
        elevation: 2,
        highlightElevation: 4,
        backgroundColor: scheme.primary,
        foregroundColor: scheme.onPrimary,
        extendedTextStyle: const TextStyle(
          fontSize: 15,
          fontWeight: FontWeight.w600,
          letterSpacing: 0.1,
        ),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppTokens.radiusLg),
        ),
      ),

      dialogTheme: DialogThemeData(
        backgroundColor: isLight ? Colors.white : scheme.surfaceContainerHigh,
        surfaceTintColor: Colors.transparent,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppTokens.radiusXl),
        ),
        titleTextStyle: base.textTheme.titleLarge?.copyWith(
          fontWeight: FontWeight.w600,
        ),
      ),

      snackBarTheme: SnackBarThemeData(
        behavior: SnackBarBehavior.floating,
        backgroundColor: scheme.inverseSurface,
        contentTextStyle: TextStyle(color: scheme.onInverseSurface),
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        ),
      ),

      dividerTheme: DividerThemeData(
        color: scheme.outlineVariant.withValues(alpha: 0.6),
        space: 1,
        thickness: 1,
      ),

      listTileTheme: ListTileThemeData(
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(AppTokens.radiusMd),
        ),
      ),

      progressIndicatorTheme: ProgressIndicatorThemeData(
        color: scheme.primary,
        // A thin ring rather than a block: it does not dominate a card or a
        // header the way a wide bar does.
        linearTrackColor: scheme.surfaceContainerHighest,
        circularTrackColor: scheme.surfaceContainerHighest,
      ),
    );
  }

  static OutlineInputBorder _inputBorder(Color color, {double width = 1}) {
    return OutlineInputBorder(
      borderRadius: BorderRadius.circular(AppTokens.radiusMd),
      borderSide: BorderSide(color: color, width: width),
    );
  }

  /// Tightens the default type scale.
  ///
  /// Flutter's defaults are set for legibility at a distance; on a phone at
  /// arm's length they read as loose. Negative tracking on the large sizes
  /// compensates, which is what keeps a heading from looking airy.
  static TextTheme _textTheme(TextTheme base, ColorScheme scheme) {
    return base
        .copyWith(
          displaySmall: base.displaySmall?.copyWith(
            fontWeight: FontWeight.w700,
            letterSpacing: -0.8,
          ),
          headlineMedium: base.headlineMedium?.copyWith(
            fontWeight: FontWeight.w700,
            letterSpacing: -0.6,
          ),
          headlineSmall: base.headlineSmall?.copyWith(
            fontWeight: FontWeight.w700,
            letterSpacing: -0.4,
          ),
          titleLarge: base.titleLarge?.copyWith(
            fontWeight: FontWeight.w600,
            letterSpacing: -0.2,
          ),
          titleMedium: base.titleMedium?.copyWith(
            fontWeight: FontWeight.w600,
            letterSpacing: -0.1,
          ),
          titleSmall: base.titleSmall?.copyWith(fontWeight: FontWeight.w600),
          labelLarge: base.labelLarge?.copyWith(fontWeight: FontWeight.w600),
        )
        .apply(
          // Body text is capped slightly darker than the scheme's default so it
          // stays readable on the tinted scaffold rather than washing out.
          bodyColor: scheme.onSurface,
          displayColor: scheme.onSurface,
        );
  }
}
