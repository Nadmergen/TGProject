import unittest
from your_project.call_service import CallService

class TestCallService(unittest.TestCase):
    def test_example_function(self):
        # Arrange
        service = CallService()
        expected = 'expected_result'
        # Act
        result = service.example_function()
        # Assert
        self.assertEqual(result, expected)

if __name__ == '__main__':
    unittest.main()