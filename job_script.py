import sys

# This trivial script adds a space between each and every character
# the input file to output file should be like this:
# This content is DRM free -> T h i s c o n t e n t  i s  D R M  f r e e
def add_spaces_to_file(input_filename, output_filename):
    try:
        with open(input_filename, 'r', encoding='utf-8') as infile:
            content = infile.read()
            
        # Join every character with a space
        spaced_content = " ".join(list(content))
        
        with open(output_filename, 'w', encoding='utf-8') as outfile:
            outfile.write(spaced_content)
            
        print(f"Successfully processed '{input_filename}' to '{output_filename}'")
        
    except FileNotFoundError:
        print(f"Error: The file '{input_filename}' was not found.")
    except Exception as e:
        print(f"An error occurred: {e}")

if __name__ == "__main__":
    input_file = "job_input.txt" 
    if len(sys.argv) > 1:
        input_file = sys.argv[1]
        
    output_file = "job_output.txt"
    
    add_spaces_to_file(input_file, output_file)