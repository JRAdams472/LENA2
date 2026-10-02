/// Assistant orchestration for the mobile client — the ChangeNotifier
/// counterpart of the web's useAssistant hook. Resolves which engine
/// answers a question — the on-device Gemma runtime once the user has
/// opted in and downloaded the model, else the server — and owns the
/// opt-in/download lifecycle.
library;

import 'package:flutter/foundation.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'agent.dart';
import 'api.dart';
import 'engine.dart';
import 'model_manager.dart';

enum LocalStatus { checking, unavailable, ready, optIn, downloading }

enum AssistantMode { auto, server }

const String _modeKey = 'lena-ai-mode';

class AssistantController extends ChangeNotifier {
  AssistantController({
    required AssistantApi api,
    required LocalModelManager modelManager,
    required LocalEngine Function() engineFactory,
  })  : _api = api,
        _models = modelManager,
        _engineFactory = engineFactory;

  final AssistantApi _api;
  final LocalModelManager _models;
  final LocalEngine Function() _engineFactory;

  bool _serverAvailable = false;
  bool _modelInstalled = false;
  LocalStatus _status = LocalStatus.checking;
  AssistantMode _mode = AssistantMode.auto;
  LocalEngine? _engine;
  int? _downloadProgress;
  String? _downloadError;
  bool _sending = false;
  String? _error;
  List<AgentToolSpec>? _tools;
  String? _systemPrompt;

  bool get serverAvailable => _serverAvailable;
  bool get modelInstalled => _modelInstalled;
  LocalStatus get status => _status;
  AssistantMode get mode => _mode;
  int? get downloadProgress => _downloadProgress;
  String? get downloadError => _downloadError;
  bool get sending => _sending;
  String? get error => _error;

  /// Whether Dot can answer at all (server provider or local engine).
  bool get available =>
      _serverAvailable || _modelInstalled || _status == LocalStatus.optIn;

  /// Whether the next ask will run on-device.
  bool get localActive => _mode == AssistantMode.auto && _modelInstalled;

  /// Badge text for the screen.
  String get engineLabel =>
      localActive ? (_engine?.label ?? 'On this device') : 'Via server';

  /// Probes server availability and on-device model state. Called once from
  /// the screen's didChangeDependencies.
  Future<void> initialize() async {
    try {
      final prefs = await SharedPreferences.getInstance();
      _mode = prefs.getString(_modeKey) == 'server'
          ? AssistantMode.server
          : AssistantMode.auto;
    } catch (_) {
      // SharedPreferences is unavailable in bare widget tests — default.
      _mode = AssistantMode.auto;
    }
    try {
      _serverAvailable = await _api.aiAvailable();
    } catch (_) {
      _serverAvailable = false;
    }
    try {
      _modelInstalled = await _models.isInstalled();
    } catch (_) {
      // flutter_gemma unusable on this device — treat as "no model".
      _modelInstalled = false;
    }
    _status = _modelInstalled ? LocalStatus.ready : LocalStatus.optIn;
    notifyListeners();
  }

  Future<void> _setMode(AssistantMode m) async {
    _mode = m;
    notifyListeners();
    try {
      final prefs = await SharedPreferences.getInstance();
      await prefs.setString(_modeKey, m.name);
    } catch (_) {
      /* prefs unavailable — the in-memory mode still applies */
    }
  }

  /// "Use server answers instead" toggle.
  Future<void> setServerOnly(bool serverOnly) =>
      _setMode(serverOnly ? AssistantMode.server : AssistantMode.auto);

  /// Explicit opt-in: downloads the on-device model with progress.
  Future<void> enableLocal() async {
    _downloadError = null;
    _downloadProgress = 0;
    _status = LocalStatus.downloading;
    notifyListeners();
    try {
      await _models.install(
        onProgress: (p) {
          _downloadProgress = p;
          notifyListeners();
        },
      );
      _modelInstalled = true;
      _status = LocalStatus.ready;
      await _setMode(AssistantMode.auto);
    } catch (e) {
      // Cancellation lands here too — either way, back to opt-in.
      _status = LocalStatus.optIn;
      _downloadError = e.toString();
    } finally {
      _downloadProgress = null;
      notifyListeners();
    }
  }

  void cancelDownload() => _models.cancelInstall();

  /// Removes the on-device model; future asks fall back to the server.
  Future<void> deleteModel() async {
    await _models.uninstall();
    await _engine?.dispose();
    _engine = null;
    _modelInstalled = false;
    _status = LocalStatus.optIn;
    notifyListeners();
  }

  Future<List<AgentToolSpec>> _loadTools() async =>
      _tools ??= await _api.assistantTools();

  Future<String> _loadPrompt() async =>
      _systemPrompt ??= await _api.assistantPrompt('ask');

  /// Answers one question, preferring the local engine in auto mode and
  /// falling back to the server when local inference can't complete.
  Future<AssistantResult> ask(String question) async {
    _sending = true;
    _error = null;
    notifyListeners();
    try {
      if (_mode != AssistantMode.server && _modelInstalled) {
        _engine ??= _engineFactory();
        try {
          final tools = await _loadTools();
          final prompt = await _loadPrompt();
          final out = await runAgent(
            engine: _engine!,
            systemPrompt: prompt,
            tools: tools,
            callTool: _api.callAssistantTool,
            question: question,
          );
          return AssistantResult(
            answer: out.answer,
            tools: out.tools,
            engineLabel: _engine!.label,
            isLocal: true,
          );
        } catch (e) {
          // Local inference failed (model unloaded, OOM, protocol drift) —
          // drop the broken engine and fall back to the server for this
          // turn, mirroring the web client's behavior.
          await _engine?.dispose();
          _engine = null;
          if (!_serverAvailable) {
            throw Exception(
              "the on-device model didn't answer and no server provider "
              'is configured',
            );
          }
        }
      }
      if (!_serverAvailable) {
        throw Exception("Dot isn't configured on this server");
      }
      final out = await _api.askAssistant(question);
      return AssistantResult(
        answer: out.answer,
        tools: out.tools,
        engineLabel: 'Via server',
        isLocal: false,
      );
    } catch (e) {
      _error = e.toString();
      rethrow;
    } finally {
      _sending = false;
      notifyListeners();
    }
  }

  @override
  void dispose() {
    _engine?.dispose();
    super.dispose();
  }
}
