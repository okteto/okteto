import React, { Component } from 'react';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';
import classnames from 'classnames';

import 'components/Input.scss';

class Input extends Component {
  constructor(props) {
    super(props);
  }

  @autobind
  handleChange(event) {
    if (this.props.onChange) {
      this.props.onChange(event.target.value);
    }
  }

  @autobind
  handleKeyPress(event) {
    if (this.props.onKeyPress) {
      this.props.onKeyPress(event);
    }
  }

  render() {
    return (
      <input
        id={this.props.id || `Input-${Date.now()}-${Math.random()}`}
        className={classnames('Input', this.props.className)}
        type={this.props.type || 'text'}
        onChange={this.handleChange}
        onKeyPress={this.handleKeyPress}
        value={this.props.value}
        placeholder={this.props.placeholder}
      />
    );
  }
}

Input.defaultProps = {
  options: [],
  isSearchable: false,
  value: ''
};

Input.propTypes = {
  id: PropTypes.string,
  type: PropTypes.string,
  onChange: PropTypes.func,
  onKeyPress: PropTypes.func,
  value: PropTypes.string,
  className: PropTypes.string,
  placeholder: PropTypes.string
};

export default Input;
