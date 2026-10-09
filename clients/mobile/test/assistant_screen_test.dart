import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_gemma/flutter_gemma.dart' show ModelFileType, ModelType;
import 'package:flutter_test/flutter_test.dart';
import 'package:graphql_flutter/graphql_flutter.dart';
import 'package:lena_mobile/ai/api.dart';
import 'package:lena_mobile/ai/controller.dart';
import 'package:lena_mobile/ai/engine.dart';
import 'package:lena_mobile/ai/gemma_binding.dart';
import 'package:lena_mobile/ai/model_manager.dart';
import 'package:lena_mobile/graphql_config.dart';
import 'package:lena_mobile/screens/assistant_screen.dart';
import 'package:shared_preferences/shared_preferences.dart';

class _FakeApi implements AssistantApi {
  var available = true;
  var throwOnAsk = false;
  Completer<void>? askGate;
  Completer<void>? availabilityGate;

  @override
  Future<bool> aiAvailable() async {
    await availabilityGate?.future;
    return available;
  }

  @override
  Future<({String answer, List<String> tools})> askAssistant(
      String question) async {
    await askGate?.future;
    if (throwOnAsk) throw Exception('server exploded');
    return (answer: 'server says hi', tools: ['get_pantry_inventory']);
  }

  @override
  Future<List<AgentToolSpec>> assistantTools() async => const [];

  @override
  Future<String> assistantPrompt(String name) async => 'You are Dot.';

  @override
  Future<String> callAssistantTool(String name, String argumentsJson) async =>
      '{}';
}

class _FakeCancel implements GemmaCancelHandle {
  var cancelled = false;
  @override
  void cancel([String? reason]) => cancelled = true;
}

class _FakeBinding implements GemmaBinding {
  var installed = false;
  Completer<void>? installGate;
  _FakeCancel? lastCancel;
  var installStarted = false;

  @override
  GemmaCancelHandle newCancelHandle() => lastCancel = _FakeCancel();

  @override
  Future<bool> isModelInstalled(String modelId) async => installed;

  @override
  Future<void> installModel({
    required String url,
    ModelType modelType = ModelType.gemmaIt,
    ModelFileType fileType = ModelFileType.litertlm,
    String? token,
    void Function(int progress)? onProgress,
    GemmaCancelHandle? cancelHandle,
  }) async {
    installStarted = true;
    onProgress?.call(42);
    await installGate?.future;
    installed = true;
  }

  @override
  Future<void> uninstallModel(String modelId) async => installed = false;

  @override
  Future<GemmaSessionHandle> createSession({String? systemInstruction}) async =>
      throw UnimplementedError();
}

class _FakeEngine implements LocalEngine {
  @override
  String get id => 'gemma';
  @override
  String get label => 'On this device (Gemma)';
  @override
  Future<String> chat(List<EngineMessage> messages) async =>
      '{"answer":"local answer"}';
  @override
  Future<void> dispose() async {}
}

AssistantController _controller(_FakeApi api, _FakeBinding binding) =>
    AssistantController(
      api: api,
      modelManager: LocalModelManager(binding),
      engineFactory: () => _FakeEngine(),
    );

Future<void> _pump(WidgetTester tester, AssistantController c) async {
  await tester.pumpWidget(MaterialApp(home: AssistantScreen(controller: c)));
}

Future<void> _settle(WidgetTester tester) async {
  for (var i = 0; i < 8; i++) {
    await tester.pump(const Duration(milliseconds: 50));
  }
}

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));

  testWidgets(
    'AssistantScreen offers the on-device model when the server has no AI provider',
    (tester) async {
      SharedPreferences.setMockInitialValues({});
      await tester.pumpWidget(
        GraphQLProvider(
          client: ValueNotifier(graphQLClient),
          child: const MaterialApp(home: AssistantScreen()),
        ),
      );
      // The availability check fires on first build; the dead test endpoint
      // resolves it as a failure. With no server provider and no on-device
      // model, the screen now offers the local-model opt-in instead of a
      // dead end.
      for (var i = 0; i < 10; i++) {
        await tester.pump(const Duration(seconds: 1));
      }

      expect(find.text('Ask Dot'), findsOneWidget);
      expect(find.text('Run Dot on this device'), findsOneWidget);
      expect(find.text('Download model'), findsOneWidget);
    },
  );

  testWidgets('shows the skeleton while availability is still resolving',
      (tester) async {
    final api = _FakeApi()..availabilityGate = Completer<void>();
    await _pump(tester, _controller(api, _FakeBinding()));
    await tester.pump();

    expect(find.byType(CircularProgressIndicator), findsNothing);
    // SkeletonList — a ListView of placeholder rows.
    expect(find.byType(ListView), findsOneWidget);
  });

  testWidgets('a question sent through the field renders the server answer',
      (tester) async {
    final api = _FakeApi();
    await _pump(tester, _controller(api, _FakeBinding()));
    await _settle(tester);

    // Opt-in card for the local model is present alongside the chat.
    expect(find.text('Run Dot on this device'), findsOneWidget);
    expect(find.text('Ask Dot anything about your kitchen.'), findsOneWidget);

    await tester.enterText(find.byType(TextField), '  what is for dinner  ');
    await tester.testTextInput.receiveAction(TextInputAction.done);
    await _settle(tester);

    // Trimmed user bubble + assistant reply with tools + engine label.
    expect(find.text('what is for dinner'), findsOneWidget);
    expect(find.text('server says hi'), findsOneWidget);
    expect(find.text('looked up: get_pantry_inventory'), findsOneWidget);
    expect(find.text('Via server'), findsOneWidget);
    expect(tester.widget<TextField>(find.byType(TextField)).controller!.text,
        isEmpty);
  });

  testWidgets('empty input is not sent', (tester) async {
    final api = _FakeApi();
    await _pump(tester, _controller(api, _FakeBinding()));
    await _settle(tester);

    await tester.enterText(find.byType(TextField), '   ');
    await tester.tap(find.byIcon(Icons.send));
    await _settle(tester);

    expect(find.text('server says hi'), findsNothing);
  });

  testWidgets('tapping a starter prompt sends it', (tester) async {
    final api = _FakeApi();
    await _pump(tester, _controller(api, _FakeBinding()));
    await _settle(tester);

    await tester.tap(find.text("What's in my wine cellar?"));
    await _settle(tester);

    expect(find.text("What's in my wine cellar?"), findsWidgets);
    expect(find.text('server says hi'), findsOneWidget);
  });

  testWidgets('shows the thinking indicator and disables send while asking',
      (tester) async {
    final api = _FakeApi()..askGate = Completer<void>();
    await _pump(tester, _controller(api, _FakeBinding()));
    await _settle(tester);

    await tester.enterText(find.byType(TextField), 'hi');
    await tester.tap(find.byIcon(Icons.send));
    await tester.pump();
    await tester.pump();

    expect(find.text('Dot is thinking…'), findsOneWidget);
    expect(
      tester.widget<IconButton>(find.ancestor(
        of: find.byIcon(Icons.send),
        matching: find.byType(IconButton),
      )),
      isNotNull,
    );

    api.askGate!.complete();
    await _settle(tester);
    expect(find.text('server says hi'), findsOneWidget);
    expect(find.text('Dot is thinking…'), findsNothing);
  });

  testWidgets('a failed ask surfaces the controller error text',
      (tester) async {
    final api = _FakeApi()..throwOnAsk = true;
    await _pump(tester, _controller(api, _FakeBinding()));
    await _settle(tester);

    await tester.enterText(find.byType(TextField), 'hi');
    await tester.testTextInput.receiveAction(TextInputAction.done);
    await _settle(tester);

    expect(find.textContaining('server exploded'), findsOneWidget);
  });

  testWidgets('download model shows progress then the local engine badge',
      (tester) async {
    final api = _FakeApi();
    final binding = _FakeBinding()..installGate = Completer<void>();
    await _pump(tester, _controller(api, binding));
    await _settle(tester);

    await tester.tap(find.text('Download model'));
    await tester.pump();
    await tester.pump();

    expect(find.text('Downloading the on-device model…'), findsOneWidget);
    expect(find.text('42%'), findsOneWidget);
    expect(find.byType(LinearProgressIndicator), findsOneWidget);
    expect(find.text('Cancel'), findsOneWidget);

    binding.installGate!.complete();
    await _settle(tester);

    // Installed → badge + popup menu appear; opt-in card is gone. The
    // badge falls back to 'On this device' until the engine exists.
    expect(find.text('On this device'), findsOneWidget);
    expect(find.text('Run Dot on this device'), findsNothing);
    expect(find.byType(PopupMenuButton<String>), findsOneWidget);
  });

  testWidgets('cancelling the download returns to the opt-in card',
      (tester) async {
    final api = _FakeApi();
    final binding = _FakeBinding()..installGate = Completer<void>();
    await _pump(tester, _controller(api, binding));
    await _settle(tester);

    await tester.tap(find.text('Download model'));
    await tester.pump();
    await tester.pump();
    expect(find.text('Downloading the on-device model…'), findsOneWidget);

    await tester.tap(find.text('Cancel'));
    binding.installGate!.completeError(Exception('User cancelled'));
    await _settle(tester);

    expect(binding.lastCancel!.cancelled, isTrue);
    expect(find.text('Run Dot on this device'), findsOneWidget);
    expect(find.text('Last download attempt failed.'), findsOneWidget);
  });

  testWidgets('the engine menu toggles server mode and deletes the model',
      (tester) async {
    final api = _FakeApi();
    final binding = _FakeBinding()..installed = true;
    await _pump(tester, _controller(api, binding));
    await _settle(tester);

    // Ready + local → badge shows the engine label.
    expect(find.text('On this device'), findsOneWidget);

    // Switch to server answers.
    await tester.tap(find.byType(PopupMenuButton<String>));
    await _settle(tester);
    await tester.tap(find.text('Use server answers'));
    await _settle(tester);
    expect(find.text('Via server'), findsOneWidget);

    // Switch back to local.
    await tester.tap(find.byType(PopupMenuButton<String>));
    await _settle(tester);
    await tester.tap(find.text('Use on-device model'));
    await _settle(tester);
    expect(find.text('On this device'), findsOneWidget);

    // Delete the model → back to the opt-in card.
    await tester.tap(find.byType(PopupMenuButton<String>));
    await _settle(tester);
    await tester.tap(find.text('Delete on-device model'));
    await _settle(tester);
    expect(find.text('Run Dot on this device'), findsOneWidget);
    expect(binding.installed, isFalse);
  });

  testWidgets('an installed model answers on-device', (tester) async {
    final api = _FakeApi();
    final binding = _FakeBinding()..installed = true;
    await _pump(tester, _controller(api, binding));
    await _settle(tester);

    await tester.enterText(find.byType(TextField), 'hi');
    await tester.testTextInput.receiveAction(TextInputAction.done);
    await _settle(tester);

    expect(find.text('local answer'), findsOneWidget);
  });
}
