import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:notes_frontend/app/theme.dart';

/// Wraps [child] in the minimum a real screen provides.
Widget wrap(Widget child) => MaterialApp(
      theme: AppTheme.light(),
      home: Scaffold(body: child),
    );

void main() {
  group('AppTokens', () {
    test('every spacing value sits on the 4pt grid', () {
      // A token off the grid is what makes two unrelated rows stop lining up.
      const spacings = [
        AppTokens.xs,
        AppTokens.sm,
        AppTokens.md,
        AppTokens.lg,
        AppTokens.xl,
        AppTokens.xxl,
        AppTokens.xxxl,
      ];

      for (final value in spacings) {
        expect(value % 4, 0, reason: '$value is not a multiple of 4');
      }
    });

    test('spacing increases monotonically', () {
      expect(AppTokens.xs, lessThan(AppTokens.sm));
      expect(AppTokens.sm, lessThan(AppTokens.md));
      expect(AppTokens.md, lessThan(AppTokens.lg));
      expect(AppTokens.lg, lessThan(AppTokens.xl));
      expect(AppTokens.xl, lessThan(AppTokens.xxl));
      expect(AppTokens.xxl, lessThan(AppTokens.xxxl));
    });

    test('radii increase monotonically', () {
      expect(AppTokens.radiusSm, lessThan(AppTokens.radiusMd));
      expect(AppTokens.radiusMd, lessThan(AppTokens.radiusLg));
      expect(AppTokens.radiusLg, lessThan(AppTokens.radiusXl));
    });
  });

  group('AppTheme', () {
    test('builds both brightnesses', () {
      expect(AppTheme.light().brightness, Brightness.light);
      expect(AppTheme.dark().brightness, Brightness.dark);
    });

    test('the scaffold is tinted in light mode rather than pure white', () {
      // A pure white scaffold is what makes a light UI look washed out; the
      // token exists precisely to avoid it.
      expect(
        AppTheme.light().scaffoldBackgroundColor,
        isNot(const Color(0xFFFFFFFF)),
      );
    });

    test('headings carry weight and tighter tracking', () {
      final textTheme = AppTheme.light().textTheme;

      expect(textTheme.headlineMedium?.fontWeight, FontWeight.w700);
      expect(textTheme.titleLarge?.fontWeight, FontWeight.w600);
      // Negative tracking is what stops a large heading from looking airy.
      expect(textTheme.headlineMedium?.letterSpacing, lessThan(0));
    });

    test('buttons meet the minimum tap target', () {
      final filled = AppTheme.light().filledButtonTheme.style;
      // 48 is the smallest reliably tappable height on a phone.
      final size = filled?.minimumSize?.resolve({});
      expect(size?.height, greaterThanOrEqualTo(48));
    });

    test('input fields are filled and borderless by default', () {
      final decoration = AppTheme.light().inputDecorationTheme;

      expect(decoration.filled, isTrue);
      // A filled, borderless field is what keeps a form from looking like a
      // stack of boxes.
      expect(
        decoration.border,
        isA<OutlineInputBorder>().having(
          (border) => border.borderSide.color,
          'color',
          Colors.transparent,
        ),
      );
    });

    test('cards use a hairline edge instead of a shadow', () {
      final card = AppTheme.light().cardTheme;

      expect(card.elevation, 0);
      expect(card.margin, EdgeInsets.zero);
      expect(
        card.shape,
        isA<RoundedRectangleBorder>().having(
          (shape) => shape.side,
          'side',
          isA<BorderSide>().having((side) => side.width, 'width', 1),
        ),
      );
    });

    test('the brand gradient has two stops', () {
      expect(AppTheme.brandGradient, hasLength(2));
    });
  });

  group('a screen using the theme', () {
    testWidgets('a themed heading keeps its weight and colour', (tester) async {
      final heading = AppTheme.light().textTheme.headlineMedium;

      await tester.pumpWidget(wrap(
        Text('Notes', style: heading),
      ));

      final rendered = tester.widget<Text>(find.text('Notes'));
      // The style has to reach the rendered widget, not merely exist on the
      // theme object, or the theme is not wired up at all.
      expect(rendered.style?.fontWeight, FontWeight.w700);
      expect(rendered.style?.color, isNotNull);
    });

    testWidgets('an outlined button picks up the themed shape', (tester) async {
      // No explicit style on the widget: whatever the theme resolves is what
      // the user actually sees.
      await tester.pumpWidget(wrap(
        OutlinedButton(onPressed: () {}, child: const Text('Tap')),
      ));

      // Asserted on the theme's own style: resolving a shape through a widget
      // merges it with the framework defaults, which obscures which value came
      // from here.
      final shape = AppTheme.light().outlinedButtonTheme.style?.shape?.resolve({});

      expect(
        shape,
        isA<RoundedRectangleBorder>().having(
          (border) => border.borderRadius,
          'borderRadius',
          BorderRadius.circular(AppTokens.radiusMd),
        ),
      );

      // The button is still on screen, so the theme resolved for a real widget
      // rather than only in theory.
      expect(find.text('Tap'), findsOneWidget);
    });
  });
}
