import 'package:flutter_gemma/flutter_gemma.dart' show ModelFileType, ModelType;
import 'package:flutter_test/flutter_test.dart';
import 'package:lena_mobile/ai/api.dart';
import 'package:lena_mobile/ai/controller.dart';
import 'package:lena_mobile/ai/engine.dart';
import 'package:lena_mobile/ai/gemma_binding.dart';
import 'package:lena_mobile/ai/model_manager.dart';
import 'package:shared_preferences/shared_preferences.dart';

const _tool = AgentToolSpec(
  name: 'get_pantry_inventory',
  description: 'Read pantry items',
  parametersJson: '{"type":"object","properties":{}}',
);

class _FakeApi implements AssistantApi {
  var available = true;
  var askCalls = 0;
  var toolCalls = <String>[];

  @override
  Future<bool> aiAvailable() async => available;

  @override
  Future<({String answer, List<String> tools})> askAssistant(
      String question) async {
    askCalls++;
    return (answer: 'server says hi', tools: ['get_pantry_inventory']);
  }

  @override
  Future<List<AgentToolSpec>> assistantTools() async => [_tool];

  @override
  Future<String> assistantPrompt(String name) async => 'You are Dot.';

  @override
  Future<String> callAssistantTool(String name, String argumentsJson) async {
    toolCalls.add(name);
    return '{"ok":true}';
  }
}

class _FakeCancel implements GemmaCancelHandle {
  var cancelled = false;
  @override
  void cancel([String? reason]) => cancelled = true;
}

class _FakeBinding implements GemmaBinding {
  var installed = false;
  var failInstall = false;
  var installStarted = false;
  final progressSeen = <int>[];
  _FakeCancel? lastCancel;

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
    if (failInstall) throw Exception('disk full');
    onProgress?.call(10);
    onProgress?.call(100);
    installed = true;
  }

  @override
  Future<void> uninstallModel(String modelId) async => installed = false;

  @override
  Future<GemmaSessionHandle> createSession({String? systemInstruction}) async =>
      throw UnimplementedError();
}

class _FakeEngine implements LocalEngine {
  _FakeEngine(this.replies);
  final List<String> replies;
  var disposed = false;

  @override
  String get id => 'gemma';
  @override
  String get label => 'On this device (Gemma)';
  @override
  Future<String> chat(List<EngineMessage> messages) async =>
      replies.isEmpty ? '{"answer":"local answer"}' : replies.removeAt(0);
  @override
  Future<void> dispose() async => disposed = true;
}

AssistantController _controller(
        _FakeApi api, _FakeBinding binding, _FakeEngine engine) =>
    AssistantController(
      api: api,
      modelManager: LocalModelManager(binding),
      engineFactory: () => engine,
    );

void main() {
  setUp(() => SharedPreferences.setMockInitialValues({}));

  test('initialize resolves opt-in when no model is installed', () async {
    final c = _controller(_FakeApi(), _FakeBinding(), _FakeEngine([]));
    await c.initialize();
    expect(c.status, LocalStatus.optIn);
    expect(c.available, isTrue);
    expect(c.localActive, isFalse);
  });

  test('initialize resolves ready when the model exists', () async {
    final binding = _FakeBinding()..installed = true;
    final c = _controller(_FakeApi(), binding, _FakeEngine([]));
    await c.initialize();
    expect(c.status, LocalStatus.ready);
    expect(c.localActive, isTrue);
  });

  test('enableLocal downloads with progress and flips to ready', () async {
    final binding = _FakeBinding();
    final c = _controller(_FakeApi(), binding, _FakeEngine([]));
    await c.initialize();
    final progress = <int?>[];
    c.addListener(() => progress.add(c.downloadProgress));
    await c.enableLocal();
    expect(binding.installStarted, isTrue);
    expect(binding.installed, isTrue);
    expect(c.status, LocalStatus.ready);
    expect(progress, contains(10));
    expect(progress, contains(100));
  });

  test('enableLocal returns to opt-in on failure', () async {
    final binding = _FakeBinding()..failInstall = true;
    final c = _controller(_FakeApi(), binding, _FakeEngine([]));
    await c.initialize();
    await c.enableLocal();
    expect(c.status, LocalStatus.optIn);
    expect(c.downloadError, contains('disk full'));
    expect(c.available, isTrue); // server still available
  });

  test('ask runs locally via the agent when a model is installed', () async {
    final api = _FakeApi();
    final binding = _FakeBinding()..installed = true;
    final engine = _FakeEngine(['{"answer":"local answer"}']);
    final c = _controller(api, binding, engine);
    await c.initialize();
    final out = await c.ask('q');
    expect(out.answer, 'local answer');
    expect(out.isLocal, isTrue);
    expect(out.engineLabel, 'On this device (Gemma)');
    expect(api.askCalls, 0);
  });

  test('ask falls back to the server when the local engine fails', () async {
    final api = _FakeApi();
    final binding = _FakeBinding()..installed = true;
    // Engine that always throws:
    final broken = _BrokenEngine();
    final c = AssistantController(
      api: api,
      modelManager: LocalModelManager(binding),
      engineFactory: () => broken,
    );
    await c.initialize();
    final out = await c.ask('q');
    expect(out.isLocal, isFalse);
    expect(out.answer, 'server says hi');
    expect(api.askCalls, 1);
    expect(broken.disposed, isTrue);
  });

  test('server mode bypasses the local engine entirely', () async {
    final api = _FakeApi();
    final binding = _FakeBinding()..installed = true;
    final engine = _FakeEngine(['{"answer":"local"}']);
    final c = _controller(api, binding, engine);
    await c.initialize();
    await c.setServerOnly(true);
    expect(c.localActive, isFalse);
    expect(c.engineLabel, 'Via server');
    final out = await c.ask('q');
    expect(out.isLocal, isFalse);
    expect(api.askCalls, 1);
  });

  test('deleteModel returns to opt-in and disposes the engine', () async {
    final api = _FakeApi();
    final binding = _FakeBinding()..installed = true;
    final engine = _FakeEngine([]);
    final c = _controller(api, binding, engine);
    await c.initialize();
    await c.ask('q'); // creates the engine lazily
    await c.deleteModel();
    expect(binding.installed, isFalse);
    expect(c.status, LocalStatus.optIn);
    expect(engine.disposed, isTrue);
  });

  test('unavailable when server is off and no model is installable path',
      () async {
    // status stays optIn (download offered) — available is still true since
    // the user can opt in; verify server-off shows via engineLabel path.
    final api = _FakeApi()..available = false;
    final c = _controller(api, _FakeBinding(), _FakeEngine([]));
    await c.initialize();
    expect(c.serverAvailable, isFalse);
    expect(c.status, LocalStatus.optIn);
  });
}

class _BrokenEngine implements LocalEngine {
  var disposed = false;
  @override
  String get id => 'gemma';
  @override
  String get label => 'On this device (Gemma)';
  @override
  Future<String> chat(List<EngineMessage> messages) async =>
      throw Exception('model crashed');
  @override
  Future<void> dispose() async => disposed = true;
}
