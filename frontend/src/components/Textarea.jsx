import React, { Component } from 'react';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';

import 'components/Textarea.scss';

class Textarea extends Component {
  constructor(props) {
    super(props);
  }

  @autobind
  onChange(event) {
    if (this.props.onChange) {
      this.props.onChange(event.target.value);
    }
  }

  render() {
    return (
      <textarea
        id={this.props.id || `Textarea-${Date.now()}-${Math.random()}`}
        className="Textarea"
        style={{
          resize: this.props.resize,
          height: `${this.props.height}px`
        }}
        onChange={this.onChange}
        value={this.props.value}
      ></textarea>
    );
  }
}

Textarea.defaultProps = {
  options: [],
  isSearchable: false,
  value: '',
  resize: 'both'
};

Textarea.propTypes = {
  id: PropTypes.string,
  onChange: PropTypes.func,
  value: PropTypes.string,
  resize: PropTypes.string,
  height: PropTypes.number
};

export default Textarea;
