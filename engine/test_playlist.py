"""
Tests for playlist.py — song_list.txt management.
"""
import os
import tempfile
import unittest

from playlist import append_song, read_songs, remove_song, update_song


class TestPlaylist(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        self.list_path = os.path.join(self.tmp, "song_list.txt")

    def tearDown(self):
        for f in os.listdir(self.tmp):
            os.remove(os.path.join(self.tmp, f))
        os.rmdir(self.tmp)

    def _write(self, lines: list):
        with open(self.list_path, "w", encoding="utf-8") as f:
            for line in lines:
                f.write(line + "\n")

    def test_read_empty(self):
        self.assertEqual(read_songs(self.list_path), [])

    def test_read_nonexistent(self):
        self.assertEqual(read_songs("/nonexistent/path.txt"), [])

    def test_append_and_read(self):
        append_song(self.list_path, "/path/to/song.mp3", "Test Title", "Test Artist", "03:30")
        songs = read_songs(self.list_path)
        self.assertEqual(len(songs), 1)
        self.assertEqual(songs[0]["filename"], "/path/to/song.mp3")
        self.assertEqual(songs[0]["title"], "Test Title")
        self.assertEqual(songs[0]["artist"], "Test Artist")
        self.assertEqual(songs[0]["duration"], "03:30")
        self.assertIn("date_added", songs[0])

    def test_remove_song(self):
        self._write(["file1|Title1|Artist1|2024-01-01|03:00",
                     "file2|Title2|Artist2|2024-01-02|04:00"])
        result = remove_song(self.list_path, "file1")
        self.assertEqual(result["status"], "ok")
        songs = read_songs(self.list_path)
        self.assertEqual(len(songs), 1)
        self.assertEqual(songs[0]["filename"], "file2")

    def test_remove_not_found(self):
        self._write(["file1|Title1|Artist1|2024-01-01|03:00"])
        result = remove_song(self.list_path, "nonexistent")
        self.assertEqual(result["status"], "error")

    def test_remove_missing_file(self):
        result = remove_song("/nonexistent/song_list.txt", "file1")
        self.assertEqual(result["status"], "error")

    def test_update_song(self):
        self._write(["file1|Title1|Artist1|2024-01-01|03:00"])
        result = update_song(self.list_path, "file1", title="New Title", artist="New Artist")
        self.assertEqual(result["status"], "ok")
        songs = read_songs(self.list_path)
        self.assertEqual(songs[0]["title"], "New Title")
        self.assertEqual(songs[0]["artist"], "New Artist")
        self.assertEqual(songs[0]["duration"], "03:00")

    def test_update_partial(self):
        self._write(["file1|Title1|Artist1|2024-01-01|03:00"])
        update_song(self.list_path, "file1", title="Only Title")
        songs = read_songs(self.list_path)
        self.assertEqual(songs[0]["title"], "Only Title")
        self.assertEqual(songs[0]["artist"], "Artist1")

    def test_update_not_found(self):
        self._write(["file1|Title1|Artist1|2024-01-01|03:00"])
        result = update_song(self.list_path, "nonexistent", title="X")
        self.assertEqual(result["status"], "error")

    def test_update_missing_file(self):
        result = update_song("/nonexistent/song_list.txt", "file1", title="X")
        self.assertEqual(result["status"], "error")


if __name__ == "__main__":
    unittest.main()
