import 'dart:convert';
import 'dart:io';

import 'package:media_kit/media_kit.dart';

import 'core_bridge.dart';
import 'media_pipeline.dart';

Future<void> runPackageSmoke(List<String> arguments) async {
  if (arguments.length != 3) exit(2);
  final report = File(arguments[1]), input = arguments[2];
  Player? player;
  try {
    final repository = NativeRepository();
    await repository.initialize();
    final executor = FFmpegExecutor();
    final probe = await executor.probe(input);
    verifyMediaDuration(probe, 3);
    final remuxed = File(
      '${report.parent.path}${Platform.pathSeparator}remuxed.mkv',
    );
    await executor.run([
      '-i',
      input,
      '-map',
      '0:v:0',
      '-c',
      'copy',
      remuxed.path,
    ]);
    verifyMediaDuration(await executor.probe(remuxed.path), 3);
    player = Player(
      configuration: const PlayerConfiguration(muted: true, vo: 'null'),
    );
    await player.setAudioTrack(AudioTrack.no());
    var position = Duration.zero;
    var duration = Duration.zero;
    final positionStream = player.stream.position.listen(
      (value) => position = value,
    );
    final durationStream = player.stream.duration.listen(
      (value) => duration = value,
    );
    await player.open(Media(Uri.file(remuxed.path).toString()));
    final deadline = DateTime.now().add(const Duration(seconds: 20));
    while (DateTime.now().isBefore(deadline) && position.inMilliseconds < 400) {
      await Future<void>.delayed(const Duration(milliseconds: 200));
    }
    await positionStream.cancel();
    await durationStream.cancel();
    if (duration.inMilliseconds < 2500 || !player.state.playing) {
      throw StateError(
        '播放器未能打开成品：position=${position.inMilliseconds}ms '
        'duration=${duration.inMilliseconds}ms playing=${player.state.playing}',
      );
    }
    await report.writeAsString(
      jsonEncode({
        'ok': true,
        'nativeCore': true,
        'ffprobe': true,
        'remux': true,
        'mediaKitPlayback': true,
        'durationMs': duration.inMilliseconds,
        'positionMs': position.inMilliseconds,
        'positionAdvanced': position.inMilliseconds >= 400,
      }),
      flush: true,
    );
    await player.dispose();
    exit(0);
  } catch (error) {
    await player?.dispose();
    await report.writeAsString(
      jsonEncode({'ok': false, 'error': error.toString()}),
      flush: true,
    );
    exit(1);
  }
}
