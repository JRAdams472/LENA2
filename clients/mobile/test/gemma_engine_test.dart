import 'package:flutter_gemma/flutter_gemma.dart' show ModelFileType, ModelType;
import 'package:flutter_test/flutter_test.dart';
import 'package:lena_mobile/ai/engine.dart';
import 'package:lena_mobile/ai/gemma_binding.dart';
import 'package:lena_mobile/ai/gemma_engine.dart';

class _FakeSession implements GemmaSessionHandle {
  final List<String> userMessages = [];
  final List<String> replies;
  var closed = false;

  _FakeSession(this.replies);

  @override
  Future<void> addUserMessage(String text) async => userMessages.add(text);

  @override
  Future<String> reply() async =>
      replies.isEmpty ? 'reply' : replies.removeAt(0);

  @override
  Future<void> close() async => closed = true;
}

class _FakeBinding implements GemmaBinding {
  _FakeSession? lastSession;
  String? lastSystemInstruction;
  var sessionsCreated = 0;
  List<String> replies = const [];

  @override
  GemmaCancelHandle newCancelHandle() => throw UnimplementedError();

  @override
  Future<bool> isModelInstalled(String modelId) async => true;

  @override
  Future<void> installModel({
    required String url,
    ModelType modelType = ModelType.gemmaIt,
    ModelFileType fileType = ModelFileType.litertlm,
    String? token,
    void Function(int progress)? onProgress,
    GemmaCancelHandle? cancelHandle,
  }) async {}

  @override
  Future<void> uninstallModel(String modelId) async {}

  @override
  Future<GemmaSessionHandle> createSession({String? systemInstruction}) async {
    sessionsCreated++;
    lastSystemInstruction = systemInstruction;
    lastSession = _FakeSession(List.of(replies));
    return lastSession!;
  }
}

void main() {
  test('id and label identify the local runtime', () {
    final engine = GemmaEngine(_FakeBinding());
    expect(engine.id, 'gemma');
    expect(engine.label, 'On this device (Gemma)');
  });

  test('first chat folds system messages into the session instruction',
      () async {
    final binding = _FakeBinding();
    final engine = GemmaEngine(binding);

    await engine.chat(const [
      EngineMessage(EngineRole.system, 'You are Dot.'),
      EngineMessage(EngineRole.system, 'Tools: none.'),
      EngineMessage(EngineRole.user, 'hello'),
    ]);

    expect(binding.sessionsCreated, 1);
    expect(binding.lastSystemInstruction, 'You are Dot.\nTools: none.');
    expect(binding.lastSession!.userMessages, ['hello']);
  });

  test('system-free chats create a session without an instruction', () async {
    final binding = _FakeBinding();
    final engine = GemmaEngine(binding);

    await engine.chat(const [EngineMessage(EngineRole.user, 'hi')]);

    expect(binding.lastSystemInstruction, isNull);
    expect(binding.lastSession!.userMessages, ['hi']);
  });

  test('follow-up chats only forward new user turns', () async {
    final binding = _FakeBinding();
    final engine = GemmaEngine(binding);

    await engine.chat(const [
      EngineMessage(EngineRole.system, 'sys'),
      EngineMessage(EngineRole.user, 'first'),
    ]);
    final out = await engine.chat(const [
      EngineMessage(EngineRole.system, 'sys'),
      EngineMessage(EngineRole.user, 'first'),
      EngineMessage(EngineRole.assistant, 'reply-1'),
      EngineMessage(EngineRole.user, 'second'),
    ]);

    expect(out, 'reply');
    expect(binding.sessionsCreated, 1);
    // Assistant turns are the session's own output — never re-fed.
    expect(binding.lastSession!.userMessages, ['first', 'second']);
  });

  test('dispose closes the session; a fresh chat reopens one', () async {
    final binding = _FakeBinding();
    final engine = GemmaEngine(binding);

    await engine.chat(const [EngineMessage(EngineRole.user, 'hi')]);
    final first = binding.lastSession!;
    await engine.dispose();
    expect(first.closed, isTrue);

    await engine.chat(const [EngineMessage(EngineRole.user, 'again')]);
    expect(binding.sessionsCreated, 2);
    expect(binding.lastSession!.userMessages, ['again']);
  });

  test('dispose before any chat is a no-op', () async {
    final engine = GemmaEngine(_FakeBinding());
    await engine.dispose();
  });
}
