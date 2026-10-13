import 'package:flutter/material.dart';
import '../responsive.dart';

/// LEN-31: the medical & allergen disclaimer. Mirrors the public web page at
/// /disclaimer — reachable from the consent sheet and the disclaimer caption
/// on recipe/cook surfaces. Signing in is not required to read it.
class DisclaimerScreen extends StatelessWidget {
  const DisclaimerScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Medical & Allergen Disclaimer')),
      body: ConstrainedContent(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(20, 16, 20, 32),
          children: const [
            _P(
              'LENA can flag allergens and dietary conflicts across recipes, '
              'meal plans, events, and grocery lists. This disclaimer explains '
              'what those features are — and what they are not.',
            ),
            _H('Information, not advice'),
            _P(
              "Allergen and dietary information in Lena's Kitchen is provided "
              'for convenience only. It is not medical, nutritional, or '
              'dietary advice, and it is not a substitute for professional '
              'judgment. If you have an allergy, intolerance, or medical '
              'condition, consult a qualified healthcare professional.',
            ),
            _H('Data may be incomplete or wrong'),
            _P(
              'Allergen flags on ingredients, items, and products are '
              'maintained by households and may be incomplete, inaccurate, or '
              'out of date. Manufacturers change formulations without notice.',
            ),
            _P(
              'Warnings do not account for cross-contamination, regional '
              'product variants, or ingredients not yet catalogued.',
            ),
            _P(
              'AI-assisted features (recipe imports, suggestions) can '
              'misclassify ingredients and must not be relied on for allergy '
              'decisions.',
            ),
            _H('Always check the label'),
            _P(
              "Before cooking or consuming anything, check the product's "
              'actual label and packaging. The label on the physical product '
              'is authoritative — an app record is not. When in doubt, do not '
              'consume it.',
            ),
            _H('Liability'),
            _P(
              "To the fullest extent permitted by law, Lena's Kitchen accepts "
              'no liability for any illness, injury, allergic reaction, or '
              'other harm arising from reliance on allergen or dietary '
              'information in the app. You use these features at your own '
              'risk.',
            ),
            _H('Your records'),
            _P(
              'Recording an allergy or dietary preference in your profile '
              'requires accepting this disclaimer once. Removing a record is '
              'always available without acceptance, and warnings you have '
              'already configured keep working either way.',
            ),
          ],
        ),
      ),
    );
  }
}

class _H extends StatelessWidget {
  const _H(this.text);
  final String text;

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.only(top: 20, bottom: 6),
        child: Text(text, style: Theme.of(context).textTheme.titleMedium),
      );
}

class _P extends StatelessWidget {
  const _P(this.text);
  final String text;

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.only(bottom: 6),
        child: Text(text, style: Theme.of(context).textTheme.bodyMedium),
      );
}
